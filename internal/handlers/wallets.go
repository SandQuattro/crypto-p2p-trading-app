package handlers

import (
	"context"
)

type WalletService interface {
	GetWalletByID(ctx context.Context, id int) (string, error)
	LoadWalletsFromDB(ctx context.Context) error
	IsOurWallet(ctx context.Context, address string) (bool, error)
	GenerateWalletForUser(ctx context.Context, userID int64) (string, error)
	TrackWalletForUser(ctx context.Context, address string, derivationPath string, userID int64) error
	TrackWallet(ctx context.Context, address string, derivationPath string) error
	GetAllTrackedWalletsForUser(ctx context.Context, userID int64) ([]string, error)
	GetAllTrackedWallets(ctx context.Context) ([]string, error)
}
