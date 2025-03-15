package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Transaction represents a blockchain transaction in our system
type Transaction struct {
	ID            int       `json:"id"`
	TxHash        string    `json:"tx_hash"`
	WalletAddress string    `json:"wallet_address"`
	Amount        string    `json:"amount"`
	BlockNumber   int64     `json:"block_number"`
	Confirmed     bool      `json:"confirmed"`
	Processed     bool      `json:"processed"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TransactionService handles blockchain transaction processing
type TransactionService struct {
	db     *pgxpool.Pool
	logger *slog.Logger
	orders *OrderService
}

// NewTransactionService creates a new transaction service
func NewTransactionService(db *pgxpool.Pool, logger *slog.Logger, orders *OrderService) *TransactionService {
	return &TransactionService{
		db:     db,
		logger: logger,
		orders: orders,
	}
}

// RecordTransaction stores a new transaction in the database
func (ts *TransactionService) RecordTransaction(ctx context.Context, txHash common.Hash, walletAddress string, amount *big.Int, blockNumber int64) error {
	// Check if transaction already exists
	var exists bool
	err := ts.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM transactions WHERE tx_hash = $1)", txHash.Hex()).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if transaction exists: %w", err)
	}

	if exists {
		ts.logger.Info("Transaction already recorded", "tx_hash", txHash.Hex())
		return nil
	}

	// Insert new transaction
	_, err = ts.db.Exec(ctx,
		"INSERT INTO transactions (tx_hash, wallet_address, amount, block_number) VALUES ($1, $2, $3, $4)",
		txHash.Hex(), walletAddress, amount.String(), blockNumber)
	if err != nil {
		return fmt.Errorf("failed to insert transaction: %w", err)
	}

	ts.logger.Info("Transaction recorded", "tx_hash", txHash.Hex(), "wallet", walletAddress, "amount", amount.String())
	return nil
}

// ConfirmTransaction marks a transaction as confirmed after required confirmations
func (ts *TransactionService) ConfirmTransaction(ctx context.Context, txHash string) error {
	_, err := ts.db.Exec(ctx, "UPDATE transactions SET confirmed = true, updated_at = NOW() WHERE tx_hash = $1", txHash)
	if err != nil {
		return fmt.Errorf("failed to confirm transaction: %w", err)
	}

	ts.logger.Info("Transaction confirmed", "tx_hash", txHash)
	return nil
}

// ProcessPendingTransactions processes all confirmed but unprocessed transactions
func (ts *TransactionService) ProcessPendingTransactions(ctx context.Context) error {
	// Start a transaction
	tx, err := ts.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // Rollback if not committed

	// Get all confirmed but unprocessed transactions
	rows, err := tx.Query(ctx,
		"SELECT id, tx_hash, wallet_address, amount FROM transactions WHERE confirmed = true AND processed = false")
	if err != nil {
		return fmt.Errorf("failed to query pending transactions: %w", err)
	}
	defer rows.Close()

	var processed int
	for rows.Next() {
		var id int
		var txHash, walletAddress, amountStr string

		if err = rows.Scan(&id, &txHash, &walletAddress, &amountStr); err != nil {
			return fmt.Errorf("failed to scan transaction: %w", err)
		}

		// Parse amount
		amount, success := new(big.Int).SetString(amountStr, 10)
		if !success {
			ts.logger.Error("Invalid amount format", "tx_hash", txHash, "amount", amountStr)
			continue
		}

		// Update orders for this wallet
		if err = ts.orders.UpdateOrderStatus(ctx, walletAddress, amount); err != nil {
			ts.logger.Error("Failed to update order status", "error", err, "tx_hash", txHash)
			continue
		}

		// Mark transaction as processed
		_, err = tx.Exec(ctx, "UPDATE transactions SET processed = true, updated_at = NOW() WHERE id = $1", id)
		if err != nil {
			ts.logger.Error("Failed to mark transaction as processed", "error", err, "tx_hash", txHash)
			continue
		}

		processed++
		ts.logger.Info("Transaction processed", "tx_hash", txHash, "wallet", walletAddress, "amount", amountStr)
	}

	// Commit the transaction if we processed anything
	if processed > 0 {
		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
		ts.logger.Info("Processed transactions", "count", processed)
	}

	return nil
}

// GetTransactionsByWallet retrieves all transactions for a specific wallet
func (ts *TransactionService) GetTransactionsByWallet(ctx context.Context, walletAddress string) ([]Transaction, error) {
	rows, err := ts.db.Query(ctx,
		"SELECT id, tx_hash, wallet_address, amount, block_number, confirmed, processed, created_at, updated_at FROM transactions WHERE wallet_address = $1 ORDER BY id DESC",
		walletAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		if err = rows.Scan(&t.ID, &t.TxHash, &t.WalletAddress, &t.Amount, &t.BlockNumber, &t.Confirmed, &t.Processed, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, t)
	}

	return transactions, nil
}
