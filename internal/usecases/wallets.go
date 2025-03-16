package usecases

import (
	"log/slog"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

type WalletService struct {
	logger *slog.Logger

	masterKey *bip32.Key
	mu        sync.Mutex
	wallets   map[string]bool // Track generated wallets
	walletsMu sync.RWMutex    // Mutex for wallets map

	transactions *TransactionService
}

func NewWalletService(logger *slog.Logger, transactions *TransactionService, seed string) (*WalletService, error) {
	seedBytes := bip39.NewSeed(seed, "")
	masterKey, err := bip32.NewMasterKey(seedBytes)
	if err != nil {
		return nil, err
	}
	return &WalletService{
		logger:       logger,
		masterKey:    masterKey,
		wallets:      make(map[string]bool),
		transactions: transactions,
	}, nil
}

// SetLogger sets the logger for the wallet service
func (ws *WalletService) SetLogger(logger *slog.Logger) {
	ws.logger = logger
}

// IsOurWallet checks if the given address belongs to our system
func (ws *WalletService) IsOurWallet(address string) bool {
	ws.walletsMu.RLock()
	defer ws.walletsMu.RUnlock()
	return ws.wallets[address]
}

func (ws *WalletService) GenerateWallet() (string, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Use a counter or random number for the index
	// This is a simplified example - you might want to store the last used index
	index := uint32(time.Now().UnixNano() % 0x80000000)

	childKey, err := ws.masterKey.NewChildKey(index)
	if err != nil {
		return "", err
	}

	privKey, err := crypto.ToECDSA(childKey.Key)
	if err != nil {
		return "", err
	}

	address := crypto.PubkeyToAddress(privKey.PublicKey).Hex()

	// Track this wallet
	ws.walletsMu.Lock()
	ws.wallets[address] = true
	ws.walletsMu.Unlock()

	ws.logger.Info("Generated new wallet", "address", address)
	return address, nil
}
