package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"

	tx "github.com/Thiht/transactor/pgx"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
	"github.com/sand/crypto-p2p-trading-app/backend/pkg/database"
)

// TransactionsRepository handles blockchain transaction processing.
type TransactionsRepository struct {
	logger *slog.Logger

	db         tx.DBGetter
	transactor *tx.Transactor

	orders *OrdersRepository
}

// NewTransactionsRepository creates a new transaction service.
func NewTransactionsRepository(logger *slog.Logger, pg *database.Postgres, orders *OrdersRepository) *TransactionsRepository {
	return &TransactionsRepository{
		logger:     logger,
		db:         pg.DBGetter,
		transactor: pg.Transactor,
		orders:     orders,
	}
}

// FindTransactionsByWallet retrieves all transactions for a specific wallet.
func (r *TransactionsRepository) FindTransactionsByWallet(ctx context.Context, walletAddress string) ([]entities.Transaction, error) {
	query := `SELECT id, tx_hash, wallet_address, amount, block_number, confirmed, processed, created_at, updated_at 
                FROM transactions 
               WHERE wallet_address = $1 
               ORDER BY id DESC
`
	rows, err := r.db(ctx).Query(ctx, query, walletAddress)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	transactions, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.Transaction])
	if err != nil {
		slog.Error("failed to collect transactions rows", "error", err)
		return nil, err
	}

	return transactions, nil
}

// InsertTransaction stores a new transaction in the database
func (r *TransactionsRepository) InsertTransaction(ctx context.Context, txHash common.Hash, walletAddress string, amount *big.Int, blockNumber int64) error {
	// Check if transaction already exists
	var exists bool

	err := r.db(ctx).QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM transactions WHERE tx_hash = $1)", txHash.Hex()).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if transaction exists: %w", err)
	}

	if exists {
		r.logger.Info("Transaction already recorded", "tx_hash", txHash.Hex())
		return nil
	}

	// Insert new transaction
	_, err = r.db(ctx).Exec(ctx,
		"INSERT INTO transactions (tx_hash, wallet_address, amount, block_number) VALUES ($1, $2, $3, $4)",
		txHash.Hex(), walletAddress, amount.String(), blockNumber)
	if err != nil {
		return fmt.Errorf("failed to insert transaction: %w", err)
	}

	r.logger.Info("Transaction recorded", "tx_hash", txHash.Hex(), "wallet", walletAddress, "amount", amount.String())

	return nil
}

// UpdateTransaction marks a transaction as confirmed after required confirmations
func (r *TransactionsRepository) UpdateTransaction(ctx context.Context, txHash string) error {
	_, err := r.db(ctx).Exec(ctx, "UPDATE transactions SET confirmed = true, updated_at = NOW() WHERE tx_hash = $1", txHash)
	if err != nil {
		return fmt.Errorf("failed to confirm transaction: %w", err)
	}

	r.logger.Info("Transaction confirmed", "tx_hash", txHash)
	return nil
}

// UpdatePendingTransactions processes all confirmed but unprocessed transactions
func (r *TransactionsRepository) UpdatePendingTransactions(ctx context.Context) error {
	// Get all confirmed but unprocessed transactions
	rows, err := r.db(ctx).Query(ctx,
		"SELECT id, tx_hash, wallet_address, amount FROM transactions WHERE confirmed = true AND processed = false")
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}

	transactions, err := pgx.CollectRows(rows, pgx.RowToStructByName[entities.ConfirmedUnprocessedTransaction])
	if err != nil {
		slog.Error("failed to collect confirmed unprocessed rows", "error", err)
		return err
	}

	processed := 0
	for _, transaction := range transactions {
		// Parse amount
		amount, success := new(big.Int).SetString(transaction.Amount, 10)
		if !success {
			r.logger.Error("Invalid amount format", "tx_hash", transaction.TxHash, "amount", transaction.Amount)
			continue
		}

		// Update orders for this wallet
		if err = r.orders.UpdateOrderStatus(ctx, transaction.WalletAddress, amount); err != nil {
			r.logger.Error("Failed to update order status", "error", err, "tx_hash", transaction.TxHash)
			continue
		}

		// Mark transaction as processed
		_, err = r.db(ctx).Exec(ctx, "UPDATE transactions SET processed = true, updated_at = NOW() WHERE id = $1", transaction.Id)
		if err != nil {
			r.logger.Error("Failed to mark transaction as processed", "error", err, "tx_hash", transaction.TxHash)
			continue
		}

		processed++
		r.logger.Info("Transaction processed", "tx_hash", transaction.TxHash, "wallet", transaction.WalletAddress, "amount", transaction.Amount)
	}

	return nil
}
