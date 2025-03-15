package usecases

import (
	"context"
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Order struct {
	ID     int    `json:"id"`
	UserID string `json:"user_id"`
	Wallet string `json:"wallet"`
	Amount string `json:"amount"`
	Status string `json:"status"`
}

type OrderService struct {
	db *pgxpool.Pool
}

func NewOrderService(db *pgxpool.Pool) *OrderService {
	return &OrderService{db: db}
}

func (os *OrderService) GetUserOrders(ctx context.Context, userID string) ([]Order, error) {
	rows, err := os.db.Query(ctx, "SELECT id, user_id, wallet, amount, status FROM orders WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		if err = rows.Scan(&order.ID, &order.UserID, &order.Wallet, &order.Amount, &order.Status); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}

func (os *OrderService) CreateOrder(ctx context.Context, userID, amount string, wallet string) error {
	_, err := os.db.Exec(ctx, "INSERT INTO orders (user_id, wallet, amount, status) VALUES ($1, $2, $3, 'pending')", userID, wallet, amount)
	return err
}

func (os *OrderService) UpdateOrderStatus(ctx context.Context, wallet string, amount *big.Int) error {
	// Start a transaction
	tx, err := os.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if not committed

	// Get all pending orders for this wallet
	rows, err := tx.Query(ctx, "SELECT id, amount FROM orders WHERE wallet = $1 AND status = 'pending' ORDER BY id", wallet)
	if err != nil {
		return fmt.Errorf("failed to query pending orders: %w", err)
	}
	defer rows.Close()

	var ordersUpdated bool
	remainingAmount := new(big.Int).Set(amount)

	for rows.Next() {
		var id int
		var orderAmountStr string

		if err = rows.Scan(&id, &orderAmountStr); err != nil {
			return fmt.Errorf("failed to scan order: %w", err)
		}

		orderAmount, success := new(big.Int).SetString(orderAmountStr, 10)
		if !success {
			return fmt.Errorf("invalid amount format in database for order %d", id)
		}

		// If we have enough to cover this order
		if remainingAmount.Cmp(orderAmount) >= 0 {
			_, err = tx.Exec(ctx, "UPDATE orders SET status = 'completed' WHERE id = $1", id)
			if err != nil {
				return fmt.Errorf("failed to update order %d: %w", id, err)
			}

			// Subtract the order amount from remaining
			remainingAmount.Sub(remainingAmount, orderAmount)
			ordersUpdated = true
		}
	}

	if !ordersUpdated {
		return fmt.Errorf("received amount is insufficient for any pending orders")
	}

	// Commit the transaction
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
