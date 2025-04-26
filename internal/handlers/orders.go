package handlers

import (
	"context"
	"time"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
)

type OrderService interface {
	GetUserOrders(ctx context.Context, userID int) ([]entities.Order, error)
	CreateOrder(ctx context.Context, userID, walletID int, amount string) error
	RemoveOldOrders(ctx context.Context, olderThan time.Duration) (int64, error)
	MarkOrderForAMLReview(ctx context.Context, orderID int, notes string) error
	GetOrderIdForWallet(ctx context.Context, walletAddress string) (int, error)
	DeleteOrder(ctx context.Context, orderID int) error
}
