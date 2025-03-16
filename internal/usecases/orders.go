package usecases

import (
	"context"
	"math/big"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
)

type OrdersRepository interface {
	FindUserOrders(ctx context.Context, userID int) ([]entities.Order, error)
	InsertOrder(ctx context.Context, userID, walletID int, amount string) error
	UpdateOrderStatus(ctx context.Context, walletID int, amount *big.Int) error
}

type OrderService struct {
	repo OrdersRepository
}

func NewOrderService(repo OrdersRepository) *OrderService {
	return &OrderService{repo: repo}
}

func (os *OrderService) GetUserOrders(ctx context.Context, userID int) ([]entities.Order, error) {
	return os.repo.FindUserOrders(ctx, userID)
}

func (os *OrderService) CreateOrder(ctx context.Context, userID, walletID int, amount string) error {
	return os.repo.InsertOrder(ctx, userID, walletID, amount)
}
