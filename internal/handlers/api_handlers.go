package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases/mocked"

	"github.com/gorilla/mux"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases"
)

type HTTPHandler struct {
	logger             *slog.Logger
	dataService        *mocked.DataService
	walletService      WalletService
	orderService       OrderService
	transactionService TransactionService
}

func NewHTTPHandler(logger *slog.Logger, dataService *mocked.DataService, walletService WalletService, orderService OrderService, transactionService TransactionService) *HTTPHandler {
	return &HTTPHandler{
		logger:             logger,
		dataService:        dataService,
		walletService:      walletService,
		orderService:       orderService,
		transactionService: transactionService,
	}
}

func (h *HTTPHandler) RegisterRoutes(router *mux.Router) {
	// API endpoints.

	// Orders
	router.HandleFunc("/orders/user", h.GetUserOrders).Methods("GET")
	router.HandleFunc("/create_order", h.CreateOrder).Methods("POST")

	// Wallets
	router.HandleFunc("/generate_wallet", h.GenerateWallet).Methods("POST")
	router.HandleFunc("/wallets/user", h.GetUserWallets).Methods("GET")

	// Transactions
	router.HandleFunc("/transactions/wallet", h.GetWalletTransactions).Methods("GET")

	// Trading, Candles
	router.HandleFunc("/api/pairs", h.GetTradingPairsHandler).Methods("GET")
	router.HandleFunc("/api/candles/{symbol}", h.GetCandlesHandler).Methods("GET")

	// Static files - register last to avoid intercepting other routes.
	fs := http.FileServer(http.Dir("./static"))
	router.PathPrefix("/").Handler(http.StripPrefix("/", fs))
}

func (h *HTTPHandler) GetUserOrders(w http.ResponseWriter, r *http.Request) {
	userIDParam := r.URL.Query().Get("user_id")
	if userIDParam == "" {
		http.Error(w, "Missing required parameters: user_id", http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(userIDParam)

	orders, err := h.orderService.GetUserOrders(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(orders)
}

func (h *HTTPHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userIDParam := r.URL.Query().Get("user_id")
	amountParam := r.URL.Query().Get("amount")

	// Validate required parameters
	if userIDParam == "" || amountParam == "" {
		http.Error(w, "Missing required parameters: user_id and amount", http.StatusBadRequest)
		return
	}

	userID, err := strconv.ParseInt(userIDParam, 10, 64)

	orders, err := h.orderService.GetUserOrders(r.Context(), int(userID))
	if err != nil {
		h.logger.Error("Error getting user orders", "error", err, "user_id", userID)
		http.Error(w, fmt.Sprintf("Failed to retrieve user orders: %v", err), http.StatusInternalServerError)
		return
	}

	var wallet string

	if len(orders) > 0 {
		wallet, err = h.walletService.GetWalletByID(r.Context(), int(userID))
		h.logger.Info("Reusing existing wallet for user", "user_id", userID, "wallet", wallet)
	} else {
		wallet, err = h.walletService.GenerateWalletForUser(r.Context(), userID)
		if err != nil {
			h.logger.Error("Error generating wallet", "error", err)
			http.Error(w, fmt.Sprintf("Failed to generate wallet: %v", err), http.StatusInternalServerError)
			return
		}
		h.logger.Info("Generated new wallet for user", "user_id", userID, "wallet", wallet)
	}

	err = h.orderService.CreateOrder(r.Context(), int(userID), amountParam, wallet)
	if err != nil {
		h.logger.Error("Error creating order", "error", err, "user_id", userID, "wallet", wallet)
		http.Error(w, fmt.Sprintf("Failed to create order: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Order created successfully", "user_id", userID, "wallet", wallet, "amount", amountParam)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"wallet": wallet,
	})
}

// GetTradingPairsHandler returns a list of trading pairs.
func (h *HTTPHandler) GetTradingPairsHandler(w http.ResponseWriter, _ *http.Request) {
	pairs := make([]map[string]any, 0, len(h.dataService.TradingPairs))

	for _, pair := range h.dataService.TradingPairs {
		pair.Mutex.RLock()
		pairData := map[string]any{
			"symbol":          pair.Symbol,
			"lastPrice":       pair.LastPrice,
			"priceChange":     pair.PriceChange,
			"ordersPerSecond": pair.OrdersPerSecond,
		}
		pair.Mutex.RUnlock()

		pairs = append(pairs, pairData)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(pairs); err != nil {
		h.logger.Error("Error encoding trading pairs", "error", err)
	}
}

// GetCandlesHandler returns candle data for a trading pair.
func (h *HTTPHandler) GetCandlesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]

	candles, err := h.dataService.GetCandleData(symbol)
	if err != nil {
		if errors.Is(err, usecases.ErrTradingPairNotFound) {
			http.Error(w, "Trading pair not found", http.StatusNotFound)
		} else {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	h.logger.Info("Sending candles", "count", len(candles), "symbol", symbol)
	w.Header().Set("Content-Type", "application/json")
	encodeErr := json.NewEncoder(w).Encode(candles)
	if encodeErr != nil {
		h.logger.Error("Error encoding candles", "error", encodeErr)
	}
}

// GetWalletTransactions returns all transactions for a specific wallet
func (h *HTTPHandler) GetWalletTransactions(w http.ResponseWriter, r *http.Request) {
	wallet := r.URL.Query().Get("wallet")
	if wallet == "" {
		http.Error(w, "Missing required parameter: wallet", http.StatusBadRequest)
		return
	}

	transactions, err := h.transactionService.GetTransactionsByWallet(r.Context(), wallet)
	if err != nil {
		h.logger.Error("Error getting wallet transactions", "error", err, "wallet", wallet)
		http.Error(w, fmt.Sprintf("Failed to retrieve transactions: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transactions)
}

// GenerateWallet generates a new wallet for a specific user
func (h *HTTPHandler) GenerateWallet(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		h.logger.Error("Error, user was not provided")
		http.Error(w, fmt.Sprintf("Failed to generate wallet"), http.StatusBadRequest)
		return
	}

	// Parse user ID to int64
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		h.logger.Error("Invalid user ID format", "error", err, "user_id", userIDStr)
		http.Error(w, "Invalid user ID format", http.StatusBadRequest)
		return
	}

	var wallet string

	// Generate wallet for the specific user
	wallet, err = h.walletService.GenerateWalletForUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("Error generating wallet", "error", err, "user_id", userID)
		http.Error(w, fmt.Sprintf("Failed to generate wallet: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Generated new wallet", "user_id", userID, "wallet", wallet)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"wallet": wallet,
		// Note: We don't have direct access to the index here, but it's stored in the database
	})
}

// GetUserWallets returns all wallets for a specific user
func (h *HTTPHandler) GetUserWallets(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")

	// Default to user ID 1 if not provided
	userID := int64(1)

	// If user ID is provided, parse it
	if userIDStr != "" {
		var err error
		userID, err = strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			h.logger.Error("Invalid user ID format", "error", err, "user_id", userIDStr)
			http.Error(w, "Invalid user ID format", http.StatusBadRequest)
			return
		}
	}

	wallets, err := h.walletService.GetAllTrackedWalletsForUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("Error getting user wallets", "error", err, "user_id", userID)
		http.Error(w, fmt.Sprintf("Failed to retrieve wallets: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to a more user-friendly format
	// Note: We don't have direct access to the index and created_at here
	// In a real implementation, you would return more detailed wallet information
	response := make([]map[string]interface{}, len(wallets))
	for i, address := range wallets {
		response[i] = map[string]interface{}{
			"address": address,
			// In a real implementation, you would include these from the database
			// "index": ...,
			// "created_at": ...,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
