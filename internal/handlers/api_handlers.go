package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strconv"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/workers"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases/mocked"

	"github.com/gorilla/mux"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases"
)

var _ OrderService = (*usecases.OrderService)(nil)

type HTTPHandler struct {
	logger             *slog.Logger
	dataService        *mocked.DataService
	walletService      workers.WalletService
	orderService       OrderService
	transactionService workers.TransactionService

	bscClient *ethclient.Client
}

func NewHTTPHandler(logger *slog.Logger, bscClient *ethclient.Client, dataService *mocked.DataService, walletService workers.WalletService, orderService OrderService, transactionService workers.TransactionService) *HTTPHandler {
	return &HTTPHandler{
		logger:             logger,
		dataService:        dataService,
		walletService:      walletService,
		orderService:       orderService,
		transactionService: transactionService,
		bscClient:          bscClient,
	}
}

func (h *HTTPHandler) RegisterRoutes(router *mux.Router) {
	// API endpoints.

	// Orders
	router.HandleFunc("/orders/user", h.GetUserOrders).Methods("GET")
	router.HandleFunc("/create_order", h.CreateOrder).Methods("POST")
	router.HandleFunc("/orders/{orderId:[0-9]+}", h.DeleteOrderHandler).Methods("DELETE")

	// Wallets
	router.HandleFunc("/wallet/generate", h.GenerateWallet).Methods("POST")
	router.HandleFunc("/wallets/user", h.GetUserWallets).Methods("GET")
	router.HandleFunc("/wallets/ids", h.GetWalletDetailsHandler).Methods("GET")
	router.HandleFunc("/wallet/balance", h.CheckWalletBalance).Methods("GET")
	router.HandleFunc("/wallet/balances", h.GetWalletBalancesHandler).Methods("GET")
	router.HandleFunc("/wallet/details", h.GetWalletDetailsHandler).Methods("GET")
	router.HandleFunc("/wallet/transfer", h.TransferFundsHandler).Methods("POST")
	router.HandleFunc("/wallets/extended", h.GetWalletDetailsExtendedHandler).Methods("GET")

	// Transactions
	router.HandleFunc("/transactions/wallet", h.GetWalletTransactions).Methods("GET")

	// Trading, Candles
	router.HandleFunc("/data/pairs", h.GetTradingPairsHandler).Methods("GET")
	router.HandleFunc("/data/candles/{symbol}", h.GetCandlesHandler).Methods("GET")

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

// GetWalletDetailsExtendedHandler returns extended wallet details including creation date
func (h *HTTPHandler) GetWalletDetailsExtendedHandler(w http.ResponseWriter, r *http.Request) {
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

	// Cast to WalletService to access the extended method
	walletService, ok := h.walletService.(*usecases.WalletService)
	if !ok {
		http.Error(w, "WalletService implementation does not support extended details", http.StatusInternalServerError)
		return
	}

	walletDetails, err := walletService.GetWalletDetailsExtendedForUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("Error getting extended wallet details", "error", err, "user_id", userID)
		http.Error(w, fmt.Sprintf("Failed to retrieve extended wallet details: %v", err), http.StatusInternalServerError)
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
	txHash, err := h.walletService.TransferFunds(r.Context(), h.bscClient, fromWalletID, toAddress, amountInt)
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

// CheckWalletBalance retrieves the balance of a wallet address
func (h *HTTPHandler) CheckWalletBalance(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "Missing wallet address parameter", http.StatusBadRequest)
		return
	}

	// Получаем конкретную реализацию WalletService для использования новых методов
	walletService, ok := h.walletService.(*usecases.WalletService)
	if !ok {
		http.Error(w, "WalletService implementation does not support balance monitoring", http.StatusInternalServerError)
		return
	}

	// Используем новый метод GetWalletBalance вместо старого CheckBalance
	balance, err := walletService.GetWalletBalance(r.Context(), address)
	if err != nil {
		h.logger.Error("Failed to get wallet balance", "error", err, "address", address)
		http.Error(w, fmt.Sprintf("Failed to get balance: %v", err), http.StatusInternalServerError)
		return
	}

	// Конвертируем значения в читаемые строки для ответа
	bnbFloat := usecases.WeiToEther(balance.NativeBalance)
	tokenFloat := usecases.WeiToEther(balance.TokenBalance)

	// Готовим ответ
	response := struct {
		Address           string `json:"address"`
		TokenBalanceWei   string `json:"token_balance_wei"`
		TokenBalanceEther string `json:"token_balance_ether"`
		BNBBalanceWei     string `json:"bnb_balance_wei"`
		BNBBalanceEther   string `json:"bnb_balance_ether"`
		Status            string `json:"status"`
		LastChecked       string `json:"last_checked"`
	}{
		Address:           balance.Address,
		TokenBalanceWei:   balance.TokenBalance.String(),
		TokenBalanceEther: tokenFloat.Text('f', 18),
		BNBBalanceWei:     balance.NativeBalance.String(),
		BNBBalanceEther:   bnbFloat.Text('f', 18),
		Status:            string(balance.Status),
		LastChecked:       balance.LastChecked.Format("2006-01-02 15:04:05"),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode balance response", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// GetWalletBalancesHandler возвращает информацию о балансах всех отслеживаемых кошельков
func (h *HTTPHandler) GetWalletBalancesHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем user_id из query параметров
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "Missing required parameter: user_id", http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		h.logger.Error("Invalid user_id format", "error", err, "user_id", userIDStr)
		http.Error(w, "Invalid user_id format", http.StatusBadRequest)
		return
	}

	// Получаем walletService как конкретную реализацию для доступа к методам мониторинга
	walletService, ok := h.walletService.(*usecases.WalletService)
	if !ok {
		http.Error(w, "WalletService implementation does not support balance monitoring", http.StatusInternalServerError)
		return
	}

	// Получаем балансы кошельков
	balances, err := walletService.GetUserWalletsBalances(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get wallet balances", "error", err)
		http.Error(w, fmt.Sprintf("Failed to get wallet balances: %v", err), http.StatusInternalServerError)
		return
	}

	// Преобразуем big.Int значения в читаемые строки для JSON
	type balanceInfo struct {
		Address           string `json:"address"`
		TokenBalanceWei   string `json:"token_balance_wei"`
		TokenBalanceEther string `json:"token_balance_ether"`
		BNBBalanceWei     string `json:"bnb_balance_wei"`
		BNBBalanceEther   string `json:"bnb_balance_ether"`
		Status            string `json:"status"`
		LastChecked       string `json:"last_checked"`
	}

	result := make(map[string]balanceInfo)
	for addr, balance := range balances {
		bnbFloat := usecases.WeiToEther(balance.NativeBalance)
		tokenFloat := usecases.WeiToEther(balance.TokenBalance)

		result[addr] = balanceInfo{
			Address:           balance.Address,
			TokenBalanceWei:   balance.TokenBalance.String(),
			TokenBalanceEther: tokenFloat.Text('f', 18),
			BNBBalanceWei:     balance.NativeBalance.String(),
			BNBBalanceEther:   bnbFloat.Text('f', 18),
			Status:            string(balance.Status),
			LastChecked:       balance.LastChecked.Format("2006-01-02 15:04:05"),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.Error("Failed to encode wallet balances", "error", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// DeleteOrderHandler handles requests to delete a pending order.
func (h *HTTPHandler) DeleteOrderHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderIDStr, ok := vars["orderId"]
	if !ok {
		h.logger.Error("Order ID not found in path parameters")
		http.Error(w, "Order ID is required", http.StatusBadRequest)
		return
	}

	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		h.logger.Error("Invalid order ID format", "error", err, "order_id", orderIDStr)
		http.Error(w, "Invalid order ID format", http.StatusBadRequest)
		return
	}

	// Call the service layer to delete the order
	err = h.orderService.DeleteOrder(r.Context(), orderID)
	if err != nil {
		// Log the internal error
		h.logger.Error("Failed to delete order", "error", err, "order_id", orderID)

		// Provide appropriate HTTP response based on the error type
		// This requires the repository/service to return specific error types
		// For now, using a generic error message, but consider refining this
		if err.Error() == fmt.Sprintf("order %d not found or not deletable", orderID) { // Example check, replace with actual error handling
			http.Error(w, err.Error(), http.StatusNotFound) // Or StatusForbidden/StatusBadRequest depending on logic
		} else {
			http.Error(w, "Failed to delete order", http.StatusInternalServerError)
		}
		return
	}

	// Respond with success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Order deleted successfully"})
}
