package handlers

import (
	"context"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases"
	"math/big"
)

type OrderService interface {
	GetUserOrders(ctx context.Context, userID string) ([]usecases.Order, error)
	CreateOrder(ctx context.Context, userID, amount string, wallet string) error
	UpdateOrderStatus(ctx context.Context, wallet string, amount *big.Int) error
}
