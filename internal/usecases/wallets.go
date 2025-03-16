package usecases

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/usecases/repository"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

type WalletService struct {
	logger *slog.Logger

	masterKey *bip32.Key
	wallets   map[string]bool // In-memory cache of tracked wallets
	walletsMu sync.RWMutex    // Mutex for wallets map

	repo *repository.WalletsRepository

	transactions *TransactionService

	mu sync.Mutex
}

func NewWalletService(
	logger *slog.Logger,
	seed string,
	transactions *TransactionService,
	walletsRepo *repository.WalletsRepository,
) (*WalletService, error) {
	seedBytes := bip39.NewSeed(seed, "")
	masterKey, err := bip32.NewMasterKey(seedBytes)
	if err != nil {
		return nil, err
	}

	ws := &WalletService{
		logger:       logger,
		masterKey:    masterKey,
		wallets:      make(map[string]bool),
		transactions: transactions,
		repo:         walletsRepo,
	}

	// Load tracked wallets from database into memory cache
	if err = ws.loadWalletsFromDB(context.Background()); err != nil {
		logger.Error("Failed to load wallets from database", "error", err)
	}

	return ws, nil
}

// loadWalletsFromDB loads all tracked wallets from the database into memory
func (ws *WalletService) loadWalletsFromDB(ctx context.Context) error {
	wallets, err := ws.repo.GetAllTrackedWallets(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tracked wallets: %w", err)
	}

	ws.walletsMu.Lock()
	defer ws.walletsMu.Unlock()

	for _, wallet := range wallets {
		ws.wallets[wallet.Address] = true
	}

	ws.logger.Info("Loaded wallets from database", "count", len(wallets))
	return nil
}

// IsOurWallet checks if the given address belongs to our system
func (ws *WalletService) IsOurWallet(ctx context.Context, address string) (bool, error) {
	// First check in-memory cache for performance
	ws.walletsMu.RLock()
	cached, exists := ws.wallets[address]
	ws.walletsMu.RUnlock()

	if exists {
		return cached, nil
	}

	// If not in cache, check database
	tracked, err := ws.repo.IsWalletTracked(ctx, address)
	if err != nil {
		return false, err
	}

	// Update cache if found in database
	if tracked {
		ws.walletsMu.Lock()
		ws.wallets[address] = true
		ws.walletsMu.Unlock()
	}

	return tracked, nil
}

func (ws *WalletService) GenerateWallet(ctx context.Context) (string, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Use a counter or random number for the index
	// This is a simplified example - you might want to store the last used index
	index := uint32(time.Now().UnixNano() % 0x80000000)
	derivationPath := fmt.Sprintf("m/44'/60'/0'/0/%d", index)

	childKey, err := ws.masterKey.NewChildKey(index)
	if err != nil {
		return "", fmt.Errorf("failed to create child key: %w", err)
	}

	privKey, err := crypto.ToECDSA(childKey.Key)
	if err != nil {
		return "", fmt.Errorf("failed to convert to ECDSA: %w", err)
	}

	address := crypto.PubkeyToAddress(privKey.PublicKey).Hex()

	// Track this wallet in database
	if err = ws.repo.TrackWallet(ctx, address, derivationPath); err != nil {
		return "", fmt.Errorf("failed to track wallet: %w", err)
	}

	// Update in-memory cache
	ws.walletsMu.Lock()
	ws.wallets[address] = true
	ws.walletsMu.Unlock()

	ws.logger.Info("Generated new wallet", "address", address, "path", derivationPath)
	return address, nil
}

// GetAllTrackedWallets retrieves all tracked wallet addresses
func (ws *WalletService) GetAllTrackedWallets(ctx context.Context) ([]string, error) {
	wallets, err := ws.repo.GetAllTrackedWallets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tracked wallets: %w", err)
	}

	addresses := make([]string, len(wallets))
	for i, wallet := range wallets {
		addresses[i] = wallet.Address
	}

	return addresses, nil
}

// TrackWallet adds a wallet address to the tracking system
func (ws *WalletService) TrackWallet(ctx context.Context, address string, derivationPath string) error {
	// Track this wallet in database
	if err := ws.repo.TrackWallet(ctx, address, derivationPath); err != nil {
		return fmt.Errorf("failed to track wallet: %w", err)
	}

	// Update in-memory cache
	ws.walletsMu.Lock()
	ws.wallets[address] = true
	ws.walletsMu.Unlock()

	ws.logger.Info("Wallet added to tracking", "address", address, "path", derivationPath)
	return nil
}
