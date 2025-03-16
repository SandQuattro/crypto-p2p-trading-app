package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"sync"

	"golang.org/x/exp/maps"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
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
func (bsc *WalletService) GenerateWalletForUser(ctx context.Context, userID int64) (int, string, error) {
	if bsc.masterKey == nil {
		return 0, "", errors.New("master key not initialized")
	}

	bsc.mu.Lock()
	defer bsc.mu.Unlock()

	// Get the last used index from the database for this user
	lastIndex, err := bsc.repo.GetLastWalletIndexForUser(ctx, userID)
	if err != nil {
		return 0, "", fmt.Errorf("failed to get last wallet index for user %d: %w", userID, err)
	}

	// Increment the index for the new wallet
	newIndex := lastIndex + 1

	// Create derivation path using the user ID and index
	// Use the user ID as part of the path to ensure uniqueness
	derivationPath := fmt.Sprintf("m/44'/60'/%d'/0/%d", userID, newIndex)

	// Create a unique child key based on both user ID and index
	// This ensures different users get different wallet addresses
	childKeyIndex := uint32(userID*1000 + int64(newIndex))
	childKey, err := bsc.masterKey.NewChildKey(childKeyIndex)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create child key: %w", err)
	}

	privKey, err := crypto.ToECDSA(childKey.Key)
	if err != nil {
		return 0, "", fmt.Errorf("failed to convert to ECDSA: %w", err)
	}

	address := crypto.PubkeyToAddress(privKey.PublicKey).Hex()

	// Track this wallet in database with the user ID and index
	var walletID int
	if walletID, err = bsc.repo.TrackWalletWithUserAndIndex(ctx, address, derivationPath, userID, newIndex); err != nil {
		return 0, "", fmt.Errorf("failed to track wallet: %w", err)
	}

	// Update in-memory cache
	bsc.walletsMu.Lock()
	bsc.wallets[address] = true
	bsc.walletsMu.Unlock()

	bsc.logger.Info("Generated new wallet", "address", address, "path", derivationPath, "user", userID, "index", newIndex)
	return walletID, address, nil
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
	if _, err := bsc.repo.TrackWalletWithUserAndIndex(ctx, address, derivationPath, userID, newIndex); err != nil {
		return fmt.Errorf("failed to track wallet: %w", err)
	}

	// Update in-memory cache
	bsc.walletsMu.Lock()
	bsc.wallets[address] = true
	bsc.walletsMu.Unlock()

	return nil
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

// GetWalletDetailsForUser retrieves wallet details (ID and address) for a specific user
func (bsc *WalletService) GetWalletDetailsForUser(ctx context.Context, userID int64) ([]entities.WalletDetail, error) {
	wallets, err := bsc.repo.GetAllTrackedWalletsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	var walletDetails []entities.WalletDetail
	for _, wallet := range wallets {
		walletDetails = append(walletDetails, entities.WalletDetail{
			ID:      int64(wallet.ID),
			Address: wallet.Address,
		})
	}

	return walletDetails, nil
}

// TransferFunds transfers USDT from a deposit wallet to a destination wallet
func (bsc *WalletService) TransferFunds(ctx context.Context, fromWalletID int, toAddress string, amount *big.Int) (string, error) {
	if bsc.masterKey == nil {
		return "", errors.New("master key not initialized")
	}

	// Get wallet details from database
	wallet, err := bsc.repo.FindWalletByID(ctx, fromWalletID)
	if err != nil {
		return "", fmt.Errorf("failed to find wallet with ID %d: %w", fromWalletID, err)
	}

	if wallet == nil {
		return "", fmt.Errorf("wallet with ID %d not found", fromWalletID)
	}

	// Parse derivation path to get the child key index
	derivationPath := wallet.DerivationPath
	bsc.logger.Info("Using derivation path", "path", derivationPath, "wallet", wallet.Address)

	// Extract user ID and index from derivation path
	var userID, index int64
	_, err = fmt.Sscanf(derivationPath, "m/44'/60'/%d'/0/%d", &userID, &index)
	if err != nil {
		return "", fmt.Errorf("failed to parse derivation path: %w", err)
	}

	// Create child key index
	childKeyIndex := uint32(userID*1000 + index)
	childKey, err := bsc.masterKey.NewChildKey(childKeyIndex)
	if err != nil {
		return "", fmt.Errorf("failed to create child key: %w", err)
	}

	// Convert to ECDSA private key
	privateKey, err := crypto.ToECDSA(childKey.Key)
	if err != nil {
		return "", fmt.Errorf("failed to convert to ECDSA: %w", err)
	}

	// Connect to blockchain
	client, err := ethclient.DialContext(ctx, "https://bsc-dataseed.binance.org/")
	if err != nil {
		return "", fmt.Errorf("failed to connect to blockchain: %w", err)
	}
	defer client.Close()

	// Get the latest nonce for the sender address
	fromAddress := crypto.PubkeyToAddress(privateKey.PublicKey)
	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get gas price
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get gas price: %w", err)
	}

	// Create token transfer data
	// USDT contract address on BSC
	tokenAddress := common.HexToAddress(USDTContractAddress)

	// Create the transaction data for ERC20 transfer
	transferFnSignature := []byte("transfer(address,uint256)")
	hash := crypto.Keccak256(transferFnSignature)
	methodID := hash[:4]

	// Pad the address to 32 bytes
	paddedAddress := common.LeftPadBytes(common.HexToAddress(toAddress).Bytes(), 32)

	// Pad the amount to 32 bytes
	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)

	// Combine the method ID, address, and amount
	var data []byte
	data = append(data, methodID...)
	data = append(data, paddedAddress...)
	data = append(data, paddedAmount...)

	// Estimate gas limit
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  fromAddress,
		To:    &tokenAddress,
		Value: big.NewInt(0),
		Data:  data,
	})
	if err != nil {
		return "", fmt.Errorf("failed to estimate gas: %w", err)
	}

	// Add 20% buffer to gas limit
	gasLimit = gasLimit * 12 / 10

	// Create the transaction
	tx := types.NewTransaction(
		nonce,
		tokenAddress,
		big.NewInt(0), // We're not sending ETH, just tokens
		gasLimit,
		gasPrice,
		data,
	)

	// Get chain ID
	chainID, err := client.ChainID(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Sign the transaction
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send the transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	bsc.logger.Info("Transaction sent",
		"from", fromAddress.Hex(),
		"to", toAddress,
		"amount", amount.String(),
		"tx_hash", txHash)

	return txHash, nil
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
