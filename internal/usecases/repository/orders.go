package repository

import (
	"context"
	"errors"
	"fmt"
	tx "github.com/Thiht/transactor/pgx"
	"github.com/jackc/pgx/v5"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
	"github.com/sand/crypto-p2p-trading-app/backend/pkg/database"
	"log/slog"
	"math/big"
)

type OrdersRepository struct {
	logger *slog.Logger

	db         tx.DBGetter
	transactor *tx.Transactor
}

func NewOrdersRepository(logger *slog.Logger, pg *database.Postgres) *OrdersRepository {
	return &OrdersRepository{logger: logger, db: pg.DBGetter, transactor: pg.Transactor}
}

func (r *OrdersRepository) FindUserOrders(ctx context.Context, userID string) ([]entities.Order, error) {
	rows, err := r.db(ctx).Query(ctx, "SELECT id, user_id, wallet, amount, status FROM orders WHERE user_id = $1", userID)
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

func (r *OrdersRepository) InsertOrder(ctx context.Context, userID, amount string, wallet string) error {
	_, err := r.db(ctx).Exec(ctx, "INSERT INTO orders (user_id, wallet, amount, status) VALUES ($1, $2, $3, 'pending')", userID, wallet, amount)
	return err
}

func (r *OrdersRepository) UpdateOrderStatus(ctx context.Context, wallet string, amount *big.Int) error {
	// Start a transaction
	return r.transactor.WithinTransaction(ctx, func(ctx context.Context) error {
		// Get all pending orders for this wallet
		rows, err := r.db(ctx).Query(ctx, "SELECT id, amount FROM orders WHERE wallet = $1 AND status = 'pending' ORDER BY id", wallet)
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
				_, err = r.db(ctx).Exec(ctx, "UPDATE orders SET status = 'completed' WHERE id = $1", id)
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

		return nil
	})
}
