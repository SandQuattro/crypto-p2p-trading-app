package usecases

import (
	"context"
	"math/big"
	"time"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
)

type OrdersRepository interface {
	FindUserOrders(ctx context.Context, userID int) ([]entities.Order, error)
	InsertOrder(ctx context.Context, userID, walletID int, amount string) error
	UpdateOrderStatus(ctx context.Context, walletID int, amount *big.Int) error
	RemoveOldOrders(ctx context.Context, olderThan time.Duration) (int64, error)
	UpdateOrderAMLStatus(ctx context.Context, orderID int, status entities.AMLStatus, notes string) error
	FindOrderByWalletAddress(ctx context.Context, walletAddress string) (int, error)
	DeleteOrder(ctx context.Context, orderID int) error
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

func (os *OrderService) RemoveOldOrders(ctx context.Context, olderThan time.Duration) (int64, error) {
	return os.repo.RemoveOldOrders(ctx, olderThan)
}

func (os *OrderService) MarkOrderForAMLReview(ctx context.Context, orderID int, notes string) error {
	return os.repo.UpdateOrderAMLStatus(ctx, orderID, entities.AMLStatusFlagged, notes)
}

func (os *OrderService) GetOrderIdForWallet(ctx context.Context, walletAddress string) (int, error) {
	return os.repo.FindOrderByWalletAddress(ctx, walletAddress)
}

// DeleteOrder deletes a pending order for a specific user
func (os *OrderService) DeleteOrder(ctx context.Context, orderID int) error {
	return os.repo.DeleteOrder(ctx, orderID)
}
