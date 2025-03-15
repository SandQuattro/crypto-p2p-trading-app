package handlers

import (
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases"
)

type WebSocketHandler struct {
	logger           *slog.Logger
	dataService      *usecases.DataService
	websocketManager *Manager
}

func NewWebSocketHandler(
	logger *slog.Logger,
	dataService *usecases.DataService,
	websocketManager *Manager,
) *WebSocketHandler {
	return &WebSocketHandler{
		logger:           logger,
		dataService:      dataService,
		websocketManager: websocketManager,
	}
}

func (h *WebSocketHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/ws/{symbol}", h.HandleConnection)
}

func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]

	// Check if the trading pair exists
	_, exists := h.dataService.TradingPairs[symbol]
	if !exists {
		http.Error(w, "Trading pair not found", http.StatusNotFound)
		return
	}

	conn, err := h.websocketManager.Upgrade(w, r)
	if err != nil {
		h.logger.Error("Error upgrading connection", "error", err)
		return
	}

	h.logger.Info("New WebSocket connection", "symbol", symbol)

	// Add subscriber
	err = h.dataService.AddSubscriber(symbol, conn)
	if err != nil {
		h.logger.Error("Error adding subscriber", "error", err)
		conn.Close()
		return
	}

	// Keep connection open and handle disconnection
	for {
		_, _, readErr := conn.ReadMessage()
		if readErr != nil {
			h.logger.Error("WebSocket connection closed", "symbol", symbol, "error", readErr)
			removeErr := h.dataService.RemoveSubscriber(symbol, conn)
			if removeErr != nil {
				h.logger.Error("Error removing subscriber", "error", removeErr)
			}
			break
		}
	}
}
