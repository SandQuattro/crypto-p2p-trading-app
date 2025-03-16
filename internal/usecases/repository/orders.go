package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"

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
	rows, err := r.db(ctx).Query(ctx, "SELECT id, user_id, wallet_id, amount, status FROM orders WHERE user_id = $1", userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	orders, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Order])
	if err != nil {
		slog.Error("failed to collect orders rows", "error", err)
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
	rows, err := r.db(ctx).Query(ctx, "SELECT id, amount FROM orders WHERE wallet_id = $1 AND status = 'pending' ORDER BY id", walletID)
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

		// Convert order amount to big.Float for decimal handling
		orderAmountFloat, _, err := new(big.Float).Parse(orderAmountStr, 10)
		if err != nil {
			return fmt.Errorf("invalid amount format in database for order %d: %w", id, err)
		}

		// Convert to wei (multiply by 10^18)
		weiMultiplier := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
		orderAmountInWei := new(big.Float).Mul(orderAmountFloat, weiMultiplier)

		// Convert back to big.Int for comparison
		orderAmount := new(big.Int)
		orderAmountInWei.Int(orderAmount)

		r.logger.Info("Comparing amounts", "order_id", id, "order_amount", orderAmountStr,
			"order_amount_wei", orderAmount.String(), "transaction_amount", remainingAmount.String())

		// If we have enough to cover this order
		if remainingAmount.Cmp(orderAmount) >= 0 {
			_, err = r.db(ctx).Exec(ctx, "UPDATE orders SET status = 'completed', updated_at = NOW() WHERE id = $1", id)
			if err != nil {
				return fmt.Errorf("failed to update order %d: %w", id, err)
			}

			r.logger.Info("Order completed", "order_id", id, "wallet_id", walletID, "amount", orderAmountStr)

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
