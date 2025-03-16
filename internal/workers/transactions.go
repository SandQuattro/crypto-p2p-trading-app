package workers

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
	"math/big"
)

type TransactionService interface {
	GetTransactionsByWallet(ctx context.Context, walletAddress string) ([]entities.Transaction, error)
	RecordTransaction(ctx context.Context, txHash common.Hash, walletAddress string, amount *big.Int, blockNumber int64) error
	ConfirmTransaction(ctx context.Context, txHash string) error
	ProcessPendingTransactions(ctx context.Context) error
}
