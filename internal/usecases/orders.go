package usecases

import (
	"context"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
	"math/big"
)

type OrdersRepository interface {
	FindUserOrders(ctx context.Context, userID int) ([]entities.Order, error)
	InsertOrder(ctx context.Context, userID int, amount string, wallet string) error
	UpdateOrderStatus(ctx context.Context, wallet string, amount *big.Int) error
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

func (os *OrderService) CreateOrder(ctx context.Context, userID int, amount string, wallet string) error {
	return os.repo.InsertOrder(ctx, userID, amount, wallet)
}

func (os *OrderService) ChangeOrderStatus(ctx context.Context, wallet string, amount *big.Int) error {
	return os.repo.UpdateOrderStatus(ctx, wallet, amount)
}
