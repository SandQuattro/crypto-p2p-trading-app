package handlers

import (
	"context"
)

type WalletService interface {
	GetWalletByID(ctx context.Context, id int) (string, error)
	LoadWalletsFromDB(ctx context.Context) error
	IsOurWallet(ctx context.Context, address string) (bool, error)
	GenerateWalletForUser(ctx context.Context, userID int64) (int, string, error)
	GetAllTrackedWalletsForUser(ctx context.Context, userID int64) ([]string, error)
	GetAllTrackedWallets(ctx context.Context) ([]string, error)
}
