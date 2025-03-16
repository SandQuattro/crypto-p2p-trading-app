package usecases

import (
	"context"
	"fmt"
	"golang.org/x/exp/maps"
	"log/slog"
	"sync"

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

	// Clear the existing map and repopulate it
	maps.Clear(ws.wallets)
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

// GenerateWalletForUser generates a new wallet address for a specific user
func (ws *WalletService) GenerateWalletForUser(ctx context.Context, userID string) (string, error) {
	if userID == "" {
		userID = "default" // Use a default user ID if none is provided
	}

	ws.mu.Lock()
	defer ws.mu.Unlock()

	// Get the last used index from the database for this user
	lastIndex, err := ws.repo.GetLastWalletIndexForUser(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get last wallet index for user %s: %w", userID, err)
	}

	// Increment the index for the new wallet
	newIndex := lastIndex + 1

	// Create derivation path using the new index
	derivationPath := fmt.Sprintf("m/44'/60'/0'/0/%d", newIndex)

	childKey, err := ws.masterKey.NewChildKey(newIndex)
	if err != nil {
		return "", fmt.Errorf("failed to create child key: %w", err)
	}

	privKey, err := crypto.ToECDSA(childKey.Key)
	if err != nil {
		return "", fmt.Errorf("failed to convert to ECDSA: %w", err)
	}

	address := crypto.PubkeyToAddress(privKey.PublicKey).Hex()

	// Track this wallet in database with the user ID and index
	if err = ws.repo.TrackWalletWithUserAndIndex(ctx, address, derivationPath, userID, newIndex); err != nil {
		return "", fmt.Errorf("failed to track wallet: %w", err)
	}

	// Update in-memory cache
	ws.walletsMu.Lock()
	ws.wallets[address] = true
	ws.walletsMu.Unlock()

	ws.logger.Info("Generated new wallet", "address", address, "path", derivationPath, "user", userID, "index", newIndex)
	return address, nil
}

// GenerateWallet generates a new wallet address for the default user
func (ws *WalletService) GenerateWallet(ctx context.Context) (string, error) {
	return ws.GenerateWalletForUser(ctx, "default")
}

// TrackWalletForUser adds a wallet address to the tracking system for a specific user
func (ws *WalletService) TrackWalletForUser(ctx context.Context, address string, derivationPath string, userID string) error {
	if userID == "" {
		userID = "default" // Use a default user ID if none is provided
	}

	// Get the last used index from the database for this user
	lastIndex, err := ws.repo.GetLastWalletIndexForUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get last wallet index for user %s: %w", userID, err)
	}

	// Increment the index for the new wallet
	newIndex := lastIndex + 1

	// Track this wallet in database with the user ID and index
	if err := ws.repo.TrackWalletWithUserAndIndex(ctx, address, derivationPath, userID, newIndex); err != nil {
		return fmt.Errorf("failed to track wallet: %w", err)
	}

	// Update in-memory cache
	ws.walletsMu.Lock()
	ws.wallets[address] = true
	ws.walletsMu.Unlock()

	ws.logger.Info("Wallet added to tracking", "address", address, "path", derivationPath, "user", userID, "index", newIndex)
	return nil
}

// TrackWallet adds a wallet address to the tracking system for the default user
func (ws *WalletService) TrackWallet(ctx context.Context, address string, derivationPath string) error {
	return ws.TrackWalletForUser(ctx, address, derivationPath, "default")
}

// GetAllTrackedWalletsForUser retrieves all tracked wallet addresses for a specific user
func (ws *WalletService) GetAllTrackedWalletsForUser(ctx context.Context, userID string) ([]string, error) {
	if userID == "" {
		userID = "default" // Use a default user ID if none is provided
	}

	wallets, err := ws.repo.GetAllTrackedWalletsForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tracked wallets for user %s: %w", userID, err)
	}

	addresses := make([]string, len(wallets))
	for i, wallet := range wallets {
		addresses[i] = wallet.Address
	}

	return addresses, nil
}

// GetAllTrackedWallets retrieves all tracked wallet addresses across all users
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
