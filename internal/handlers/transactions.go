package handlers

import (
	"context"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases"
)

type TransactionService interface {
	GetTransactionsByWallet(ctx context.Context, walletAddress string) ([]usecases.Transaction, error)
}
