package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/workers"
	"log/slog"
	"math/big"
	"net/http"
	"strconv"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases/mocked"

	"github.com/gorilla/mux"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases"
)

type HTTPHandler struct {
	logger             *slog.Logger
	dataService        *mocked.DataService
	walletService      workers.WalletService
	orderService       OrderService
	transactionService workers.TransactionService
}

func NewHTTPHandler(logger *slog.Logger, dataService *mocked.DataService, walletService workers.WalletService, orderService OrderService, transactionService workers.TransactionService) *HTTPHandler {
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
	router.HandleFunc("/wallets/ids", h.GetWalletDetailsHandler).Methods("GET")
	router.HandleFunc("/transfer", h.TransferFundsHandler).Methods("POST")

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

	// Here we always generate new deposit wallet for order
	walletID, address, err := h.walletService.GenerateWalletForUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("[Create Order] Error generating wallet", "error", err)
		http.Error(w, fmt.Sprintf("Failed to generate wallet: %v", err), http.StatusInternalServerError)
		return
	}
	h.logger.Info("Generated new wallet for user", "user_id", userID, "wallet", address)

	err = h.orderService.CreateOrder(r.Context(), int(userID), walletID, amountParam)
	if err != nil {
		h.logger.Error("[Create Order] Error creating order", "error", err, "user_id", userID, "wallet", address)
		http.Error(w, fmt.Sprintf("Failed to create order: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info("[Create Order] Order created successfully", "user_id", userID, "wallet", address, "amount", amountParam)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"status":    "success",
		"wallet_id": walletID,
		"wallet":    address,
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

	// Generate wallet for the specific user
	walletID, address, err := h.walletService.GenerateWalletForUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("Error generating wallet", "error", err, "user_id", userID)
		http.Error(w, fmt.Sprintf("Failed to generate wallet: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Generated new wallet", "user_id", userID, "wallet", address)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"wallet_id": walletID,
		"wallet":    address,
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

// GetWalletDetailsHandler returns wallet details (ID and address) for a specific user
func (h *HTTPHandler) GetWalletDetailsHandler(w http.ResponseWriter, r *http.Request) {
	userIDParam := r.URL.Query().Get("user_id")
	if userIDParam == "" {
		http.Error(w, "Missing required parameter: user_id", http.StatusBadRequest)
		return
	}

	userID, err := strconv.ParseInt(userIDParam, 10, 64)
	if err != nil {
		h.logger.Error("Invalid user ID format", "error", err, "user_id", userIDParam)
		http.Error(w, "Invalid user ID format", http.StatusBadRequest)
		return
	}

	walletDetails, err := h.walletService.GetWalletDetailsForUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("Error getting wallet details", "error", err, "user_id", userID)
		http.Error(w, fmt.Sprintf("Failed to retrieve wallet details: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(walletDetails)
}

// TransferFundsHandler transfers funds from a wallet to another address
func (h *HTTPHandler) TransferFundsHandler(w http.ResponseWriter, r *http.Request) {
	// Get parameters from request
	fromWalletIDParam := r.URL.Query().Get("wallet_id")
	toAddress := r.URL.Query().Get("to_address")
	amountParam := r.URL.Query().Get("amount")

	// Validate required parameters
	if fromWalletIDParam == "" || toAddress == "" || amountParam == "" {
		http.Error(w, "Missing required parameters: wallet_id, to_address, or amount", http.StatusBadRequest)
		return
	}

	// Parse wallet ID
	fromWalletID, err := strconv.Atoi(fromWalletIDParam)
	if err != nil {
		h.logger.Error("Invalid wallet ID format", "error", err, "wallet_id", fromWalletIDParam)
		http.Error(w, "Invalid wallet ID format", http.StatusBadRequest)
		return
	}

	// Parse amount (convert from USDT to wei - multiply by 10^18)
	amountFloat, err := strconv.ParseFloat(amountParam, 64)
	if err != nil {
		h.logger.Error("Invalid amount format", "error", err, "amount", amountParam)
		http.Error(w, "Invalid amount format", http.StatusBadRequest)
		return
	}

	// Convert to wei (multiply by 10^18)
	amountWei := new(big.Float).Mul(
		big.NewFloat(amountFloat),
		new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)),
	)

	// Convert to big.Int
	amountInt := new(big.Int)
	amountWei.Int(amountInt)

	// Transfer funds
	txHash, err := h.walletService.TransferFunds(r.Context(), fromWalletID, toAddress, amountInt)
	if err != nil {
		h.logger.Error("Error transferring funds", "error", err, "from_wallet", fromWalletID, "to", toAddress, "amount", amountParam)
		http.Error(w, fmt.Sprintf("Failed to transfer funds: %v", err), http.StatusInternalServerError)
		return
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"tx_hash": txHash,
		"message": fmt.Sprintf("Successfully initiated transfer of %s USDT from wallet ID %d to %s", amountParam, fromWalletID, toAddress),
	})
}
