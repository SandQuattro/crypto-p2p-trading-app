package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	tx "github.com/Thiht/transactor/pgx"
	"github.com/jackc/pgx/v5"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
	"github.com/sand/crypto-p2p-trading-app/backend/pkg/database"
)

type OrdersRepository struct {
	logger *slog.Logger

	db         tx.DBGetter
	transactor *tx.Transactor
}

func NewOrdersRepository(logger *slog.Logger, pg *database.Postgres) *OrdersRepository {
	return &OrdersRepository{logger: logger, db: pg.DBGetter, transactor: pg.Transactor}
}

func (r *OrdersRepository) FindUserOrders(ctx context.Context, userID int) ([]entities.Order, error) {
	rows, err := r.db(ctx).Query(ctx, "SELECT id, user_id, wallet_id, amount, status, aml_status, COALESCE(aml_notes, '') as aml_notes, created_at, updated_at FROM orders WHERE user_id = $1", userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query user orders: %w", err)
	}
	defer rows.Close()

	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Order])
	if err != nil {
		r.logger.Error("failed to collect orders rows", "error", err)
		return nil, err
	}

	return orders, nil
}

func (r *OrdersRepository) InsertOrder(ctx context.Context, userID, walletID int, amount string) error {
	_, err := r.db(ctx).Exec(ctx, "INSERT INTO orders (user_id, wallet_id, amount, status) VALUES ($1, $2, $3, 'pending')", userID, walletID, amount)
	return err
}

func (r *OrdersRepository) UpdateOrderStatus(ctx context.Context, walletID int, amount *big.Int) error {
	// Get all pending orders for this wallet
	rows, err := r.db(ctx).Query(ctx, "SELECT * FROM orders WHERE wallet_id = $1 AND status = 'pending' ORDER BY id", walletID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to query pending orders by wallet id: %w", err)
	}
	defer rows.Close()

	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Order])
	if err != nil {
		r.logger.Error("failed to collect orders rows", "error", err)
		return err
	}

	var ordersUpdated bool
	remainingAmount := new(big.Int).Set(amount)

	for _, order := range orders {
		// Convert order amount to big.Float for decimal handling
		orderAmountFloat, _, err := new(big.Float).Parse(order.Amount, 10)
		if err != nil {
			return fmt.Errorf("invalid amount format in database for order %d: %w", order.ID, err)
		}

		// Convert to wei (multiply by 10^18)
		weiMultiplier := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
		orderAmountInWei := new(big.Float).Mul(orderAmountFloat, weiMultiplier)

		// Convert back to big.Int for comparison
		orderAmount := new(big.Int)
		orderAmountInWei.Int(orderAmount)

		r.logger.Info("Comparing amounts", "order_id", order.ID, "order_amount", order.Amount,
			"order_amount_wei", orderAmount.String(), "transaction_amount", remainingAmount.String())

		// If we have enough to cover this order
		if remainingAmount.Cmp(orderAmount) >= 0 {
			_, err = r.db(ctx).Exec(ctx, "UPDATE orders SET status = 'completed', updated_at = NOW() WHERE id = $1", order.ID)
			if err != nil {
				return fmt.Errorf("failed to update order %d: %w", order.ID, err)
			}

			r.logger.Info("Order completed", "order_id", order.ID, "wallet_id", walletID, "amount", order.Amount)

			// Subtract the order amount from remaining
			remainingAmount.Sub(remainingAmount, orderAmount)
			ordersUpdated = true
		}
	}

	if !ordersUpdated {
		r.logger.Warn("No orders updated", "wallet_id", walletID, "amount", amount.String())
		// Don't return an error, as this might be a legitimate case (e.g., partial payment)
		// Just log a warning instead
	}

	return nil
}

func (r *OrdersRepository) RemoveOldOrders(ctx context.Context, olderThan time.Duration) (int64, error) {
	// Calculate the cutoff time (current time - duration)
	cutoffTime := time.Now().Add(-olderThan)

	// Delete orders that are older than the cutoff time and still have 'pending' status
	result, err := r.db(ctx).Exec(ctx,
		"DELETE FROM orders WHERE status = 'pending' AND created_at < $1",
		cutoffTime)

	if err != nil {
		return 0, fmt.Errorf("failed to remove old orders: %w", err)
	}

	// Get the number of deleted rows
	deletedCount := result.RowsAffected()

	if deletedCount > 0 {
		r.logger.Info("Removed old pending orders", "count", deletedCount, "older_than", olderThan.String())
	}

	return deletedCount, nil
}

// UpdateOrderAMLStatus обновляет AML статус ордера
func (r *OrdersRepository) UpdateOrderAMLStatus(ctx context.Context, orderID int, status entities.AMLStatus, notes string) error {
	_, err := r.db(ctx).Exec(ctx,
		"UPDATE orders SET aml_status = $1, aml_notes = $2, updated_at = NOW() WHERE id = $3",
		status, notes, orderID)
	if err != nil {
		return fmt.Errorf("failed to update order AML status: %w", err)
	}

	r.logger.Info("Order AML status updated",
		"order_id", orderID,
		"status", status)
	return nil
}

// FindOrderByWalletAddress находит ID ордера по адресу кошелька
func (r *OrdersRepository) FindOrderByWalletAddress(ctx context.Context, walletAddress string) (int, error) {
	var orderID int

	query := `
		SELECT o.id 
		FROM orders o
		JOIN wallets w ON o.wallet_id = w.id
		WHERE w.address = $1 AND o.status = 'pending'
		ORDER BY o.id DESC
		LIMIT 1
	`

	err := r.db(ctx).QueryRow(ctx, query, walletAddress).Scan(&orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("no pending order found for wallet %s", walletAddress)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to find order by wallet address: %w", err)
	}

	return orderID, nil
}
