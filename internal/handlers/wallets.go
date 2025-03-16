package handlers

import "context"

type WalletService interface {
	GenerateWallet(ctx context.Context) (string, error)
	GenerateWalletForUser(ctx context.Context, userID int64) (string, error)
	GetAllTrackedWallets(ctx context.Context) ([]string, error)
	GetAllTrackedWalletsForUser(ctx context.Context, userID int64) ([]string, error)
	IsOurWallet(ctx context.Context, address string) (bool, error)
}
