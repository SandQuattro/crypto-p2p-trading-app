package handlers

import "context"

type WalletService interface {
	GenerateWallet(ctx context.Context) (string, error)
}
