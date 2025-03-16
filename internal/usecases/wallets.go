package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"golang.org/x/exp/maps"

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
func (bsc *WalletService) loadWalletsFromDB(ctx context.Context) error {
	wallets, err := bsc.repo.GetAllTrackedWallets(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tracked wallets: %w", err)
	}

	bsc.walletsMu.Lock()
	defer bsc.walletsMu.Unlock()

	// Clear the existing map and repopulate it
	maps.Clear(bsc.wallets)
	for _, wallet := range wallets {
		bsc.wallets[wallet.Address] = true
	}

	bsc.logger.Info("Loaded wallets from database", "count", len(wallets))
	return nil
}

// IsOurWallet checks if the given address belongs to our system
func (bsc *WalletService) IsOurWallet(ctx context.Context, address string) (bool, error) {
	// First check in-memory cache for performance
	bsc.walletsMu.RLock()
	cached, exists := bsc.wallets[address]
	bsc.walletsMu.RUnlock()

	if exists {
		return cached, nil
	}

	// If not in cache, check database
	tracked, err := bsc.repo.IsWalletTracked(ctx, address)
	if err != nil {
		return false, err
	}

	// Update cache if found in database
	if tracked {
		bsc.walletsMu.Lock()
		bsc.wallets[address] = true
		bsc.walletsMu.Unlock()
	}

	return tracked, nil
}

// GenerateWalletForUser generates a new wallet address for a specific user
func (bsc *WalletService) GenerateWalletForUser(ctx context.Context, userID int64) (string, error) {
	if bsc.masterKey == nil {
		return "", errors.New("master key not initialized")
	}

	bsc.mu.Lock()
	defer bsc.mu.Unlock()

	// Get the last used index from the database for this user
	lastIndex, err := bsc.repo.GetLastWalletIndexForUser(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get last wallet index for user %d: %w", userID, err)
	}

	// Increment the index for the new wallet
	newIndex := lastIndex + 1

	// Create derivation path using the new index
	derivationPath := fmt.Sprintf("m/44'/60'/0'/0/%d", newIndex)

	childKey, err := bsc.masterKey.NewChildKey(newIndex)
	if err != nil {
		return "", fmt.Errorf("failed to create child key: %w", err)
	}

	privKey, err := crypto.ToECDSA(childKey.Key)
	if err != nil {
		return "", fmt.Errorf("failed to convert to ECDSA: %w", err)
	}

	address := crypto.PubkeyToAddress(privKey.PublicKey).Hex()

	// Track this wallet in database with the user ID and index
	if err = bsc.repo.TrackWalletWithUserAndIndex(ctx, address, derivationPath, userID, newIndex); err != nil {
		return "", fmt.Errorf("failed to track wallet: %w", err)
	}

	// Update in-memory cache
	bsc.walletsMu.Lock()
	bsc.wallets[address] = true
	bsc.walletsMu.Unlock()

	bsc.logger.Info("Generated new wallet", "address", address, "path", derivationPath, "user", userID, "index", newIndex)
	return address, nil
}

// GenerateWallet generates a new wallet address for the default user (user ID 1)
func (bsc *WalletService) GenerateWallet(ctx context.Context) (string, error) {
	return bsc.GenerateWalletForUser(ctx, 1)
}

// TrackWalletForUser adds a wallet address to the tracking system for a specific user
func (bsc *WalletService) TrackWalletForUser(ctx context.Context, address string, derivationPath string, userID int64) error {
	// Get the last used index from the database for this user
	lastIndex, err := bsc.repo.GetLastWalletIndexForUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get last wallet index for user %d: %w", userID, err)
	}

	// Increment the index for the new wallet
	newIndex := lastIndex + 1

	// Track this wallet in database with the user ID and index
	if err := bsc.repo.TrackWalletWithUserAndIndex(ctx, address, derivationPath, userID, newIndex); err != nil {
		return fmt.Errorf("failed to track wallet: %w", err)
	}

	// Update in-memory cache
	bsc.walletsMu.Lock()
	bsc.wallets[address] = true
	bsc.walletsMu.Unlock()

	return nil
}

// TrackWallet adds a wallet address to the tracking system for the default user
func (bsc *WalletService) TrackWallet(ctx context.Context, address string, derivationPath string) error {
	return bsc.TrackWalletForUser(ctx, address, derivationPath, 1)
}

// GetAllTrackedWalletsForUser retrieves all tracked wallet addresses for a specific user
func (bsc *WalletService) GetAllTrackedWalletsForUser(ctx context.Context, userID int64) ([]string, error) {
	wallets, err := bsc.repo.GetAllTrackedWalletsForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tracked wallets for user %d: %w", userID, err)
	}

	addresses := make([]string, len(wallets))
	for i, wallet := range wallets {
		addresses[i] = wallet.Address
	}

	return addresses, nil
}

// GetAllTrackedWallets retrieves all tracked wallet addresses across all users
func (bsc *WalletService) GetAllTrackedWallets(ctx context.Context) ([]string, error) {
	wallets, err := bsc.repo.GetAllTrackedWallets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get tracked wallets: %w", err)
	}

	addresses := make([]string, len(wallets))
	for i, wallet := range wallets {
		addresses[i] = wallet.Address
	}

	return addresses, nil
}
