package ports

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
)

type TransactionService interface {
	GetTransactionsByWallet(ctx context.Context, walletAddress string) ([]entities.Transaction, error)
	RecordTransaction(ctx context.Context, txHash common.Hash, walletAddress string, amount *big.Int, blockNumber int64) error
	ConfirmTransaction(ctx context.Context, txHash string) error
	ProcessPendingTransactions(ctx context.Context) error
	MarkTransactionAMLFlagged(ctx context.Context, txHash string) error
	MarkTransactionAMLCleared(ctx context.Context, txHash string) error
}

// AMLService определяет интерфейс для AML проверок
type AMLService interface {
	CheckTransaction(ctx context.Context, txHash common.Hash, sourceAddress, destinationAddress string, amount *big.Int) (*entities.AMLCheckResult, error)
}

// WalletService defines the interface for wallet operations.
type WalletService interface {
	IsOurWallet(ctx context.Context, address string) (bool, error)
	GenerateWalletForUser(ctx context.Context, userID int64) (int, string, error)
	GetAllTrackedWalletsForUser(ctx context.Context, userID int64) ([]string, error)
	GetWalletDetailsForUser(ctx context.Context, userID int64) ([]entities.WalletDetail, error)
	GetERC20TokenBalance(ctx context.Context, client *ethclient.Client, walletAddress string) (*big.Int, error)
	GetGasPrice(ctx context.Context, client *ethclient.Client) (*big.Int, error)
	TransferFunds(ctx context.Context, client *ethclient.Client, fromWalletID int, toAddress string, amount *big.Int) (string, error)
	TransferAllBNB(ctx context.Context, toAddress, depositUserWalletAddress string, userID, index int) (string, error)
	GetOrderIdForWallet(ctx context.Context, walletAddress string) (int, error)
	DeleteWallet(ctx context.Context, walletID int) error

	// Методы мониторинга балансов
	GetWalletBalances(ctx context.Context) (map[string]*entities.WalletBalance, error)
	GetUserWalletsBalances(ctx context.Context, userID int) (map[string]*entities.WalletBalance, error)
	GetWalletBalance(ctx context.Context, address string) (*entities.WalletBalance, error)
}

// OrderService defines the interface for order operations.
type OrderService interface {
	GetUserOrders(ctx context.Context, userID int) ([]entities.Order, error)
	CreateOrder(ctx context.Context, userID, walletID int, amount string) error
	RemoveOldOrders(ctx context.Context, olderThan time.Duration) (int64, error)
	MarkOrderForAMLReview(ctx context.Context, orderID int, notes string) error
	MarkOrderAMLCleared(ctx context.Context, orderID int, notes string) error
	GetOrderIdForWallet(ctx context.Context, walletAddress string) (int, error)
	DeleteOrder(ctx context.Context, orderID int) error
}
