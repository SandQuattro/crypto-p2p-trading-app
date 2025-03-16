package handlers

import (
	"context"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
	"math/big"
)

type OrderService interface {
	GetUserOrders(ctx context.Context, userID int) ([]entities.Order, error)
	CreateOrder(ctx context.Context, userID int, amount string, wallet string) error
	ChangeOrderStatus(ctx context.Context, wallet string, amount *big.Int) error
}
