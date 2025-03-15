package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases"
)

type HTTPHandler struct {
	logger             *slog.Logger
	dataService        *usecases.DataService
	walletService      WalletService
	orderService       OrderService
	transactionService TransactionService
}

func NewHTTPHandler(logger *slog.Logger, dataService *usecases.DataService, walletService WalletService, orderService OrderService, transactionService TransactionService) *HTTPHandler {
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
	userID := r.URL.Query().Get("user_id")
	orders, err := h.orderService.GetUserOrders(r.Context(), userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(orders)
}

func (h *HTTPHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	amount := r.URL.Query().Get("amount")

	// Validate required parameters
	if userID == "" || amount == "" {
		http.Error(w, "Missing required parameters: user_id and amount", http.StatusBadRequest)
		return
	}

	orders, err := h.orderService.GetUserOrders(r.Context(), userID)
	if err != nil {
		h.logger.Error("Error getting user orders", "error", err, "user_id", userID)
		http.Error(w, fmt.Sprintf("Failed to retrieve user orders: %v", err), http.StatusInternalServerError)
		return
	}

	var wallet string

	if len(orders) > 0 {
		wallet = orders[0].Wallet
		h.logger.Info("Reusing existing wallet for user", "user_id", userID, "wallet", wallet)
	} else {
		var err error
		wallet, err = h.walletService.GenerateWallet()
		if err != nil {
			h.logger.Error("Error generating wallet", "error", err)
			http.Error(w, fmt.Sprintf("Failed to generate wallet: %v", err), http.StatusInternalServerError)
			return
		}
		h.logger.Info("Generated new wallet for user", "user_id", userID, "wallet", wallet)
	}

	err = h.orderService.CreateOrder(r.Context(), userID, amount, wallet)
	if err != nil {
		h.logger.Error("Error creating order", "error", err, "user_id", userID, "wallet", wallet)
		http.Error(w, fmt.Sprintf("Failed to create order: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Order created successfully", "user_id", userID, "wallet", wallet, "amount", amount)

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
