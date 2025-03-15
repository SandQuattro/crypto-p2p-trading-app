package handlers

type WalletService interface {
	GenerateWallet() (string, error)
	// We don't need to expose SubscribeToTransactions in the interface
	// since it's only called from main.go
}
