package usecases

import (
	"context"
	"math/big"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"

	"github.com/ethereum/go-ethereum/common"
)

type TransactionsRepository interface {
	FindTransactionsByWallet(ctx context.Context, walletAddress string) ([]entities.Transaction, error)
	InsertTransaction(ctx context.Context, txHash common.Hash, walletAddress string, amount *big.Int, blockNumber int64) error
	UpdateTransaction(ctx context.Context, txHash string) error
	UpdatePendingTransactions(ctx context.Context) error
}

// TransactionServiceImpl handles blockchain transaction processing
type TransactionServiceImpl struct {
	repo TransactionsRepository
}

// NewTransactionService creates a new transaction service
func NewTransactionService(repo TransactionsRepository) *TransactionServiceImpl {
	return &TransactionServiceImpl{
		repo: repo,
	}
}

// GetTransactionsByWallet retrieves all transactions for a specific wallet.
func (ts *TransactionServiceImpl) GetTransactionsByWallet(ctx context.Context, walletAddress string) ([]entities.Transaction, error) {
	return ts.repo.FindTransactionsByWallet(ctx, walletAddress)
}

// RecordTransaction stores a new transaction in the database
func (ts *TransactionServiceImpl) RecordTransaction(ctx context.Context, txHash common.Hash, walletAddress string, amount *big.Int, blockNumber int64) error {
	return ts.repo.InsertTransaction(ctx, txHash, walletAddress, amount, blockNumber)
}

// ConfirmTransaction marks a transaction as confirmed after required confirmations
func (ts *TransactionServiceImpl) ConfirmTransaction(ctx context.Context, txHash string) error {
	return ts.repo.UpdateTransaction(ctx, txHash)
}

// ProcessPendingTransactions processes all confirmed but unprocessed transactions
func (ts *TransactionServiceImpl) ProcessPendingTransactions(ctx context.Context) error {
	return ts.repo.UpdatePendingTransactions(ctx)
}
