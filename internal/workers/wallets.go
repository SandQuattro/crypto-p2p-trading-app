package workers

import (
	"context"
	"math/big"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
)

// WalletService defines the interface for wallet operations
type WalletService interface {
	IsOurWallet(ctx context.Context, address string) (bool, error)
	GenerateWalletForUser(ctx context.Context, userID int64) (int, string, error)
	GetAllTrackedWalletsForUser(ctx context.Context, userID int64) ([]string, error)
	GetWalletDetailsForUser(ctx context.Context, userID int64) ([]entities.WalletDetail, error)
	TransferFunds(ctx context.Context, fromWalletID int, toAddress string, amount *big.Int) (string, error)
	EnsureSufficientBNB(ctx context.Context, walletAddress string) error
}
