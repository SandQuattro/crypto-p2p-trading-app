package handlers

import (
	"context"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
)

type OrderService interface {
	GetUserOrders(ctx context.Context, userID int) ([]entities.Order, error)
	CreateOrder(ctx context.Context, userID, walletID int, amount string) error
}
