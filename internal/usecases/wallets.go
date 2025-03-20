package usecases

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/google/uuid"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/workers"

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

// Константы для логирования
const (
	// Статусы операций
	StatusSuccess = "success"
	StatusFailure = "failure"
	StatusPending = "pending"
)

type WalletsRepository interface {
	FindWalletByAddress(ctx context.Context, address string) (*entities.Wallet, error)
	FindWalletByID(ctx context.Context, id int) (*entities.Wallet, error)
	IsWalletTracked(ctx context.Context, address string) (bool, error)
	GetAllTrackedWallets(ctx context.Context) ([]entities.Wallet, error)
	GetLastWalletIndexForUser(ctx context.Context, userID int64) (uint32, error)
	TrackWalletWithUserAndIndex(ctx context.Context, address string, derivationPath string, userID int64, index uint32) (int, error)
	GetAllTrackedWalletsForUser(ctx context.Context, userID int64) ([]entities.Wallet, error)
}

var _ WalletsRepository = (*repository.WalletsRepository)(nil)

type WalletService struct {
	logger *slog.Logger

	erc20ABI, smartContractAddress string

	seed      string
	masterKey *bip32.Key
	wallets   map[string]bool // In-memory cache of tracked wallets
	walletsMu sync.RWMutex    // Mutex for wallets map

	repo WalletsRepository

	transactions *TransactionServiceImpl

	mu sync.Mutex
}

func NewWalletService(
	logger *slog.Logger,
	seed string,
	transactions *TransactionServiceImpl,
	walletsRepo *repository.WalletsRepository,
) (*WalletService, error) {
	ws := &WalletService{
		logger: logger,

		erc20ABI:             `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"type":"function"}]`,
		smartContractAddress: "0x55d398326f99059fF775485246999027B3197955",

		seed:         seed,
		masterKey:    CreateMasterKey(seed),
		wallets:      make(map[string]bool),
		transactions: transactions,
		repo:         walletsRepo,
	}

	// Load tracked wallets from database into memory cache
	if err := ws.loadWalletsFromDB(context.Background()); err != nil {
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

	// Get child key and private key
	childKey, err := GetChildKey(bsc.masterKey, userID, int64(newIndex))
	if err != nil {
		return 0, "", err
	}

	_, walletAddress, err := GetWalletPrivateKey(childKey)
	if err != nil {
		return 0, "", err
	}

	address := walletAddress.Hex()

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

// GetERC20TokenBalance retrieves the balance of ERC20 token for an address
func (bsc *WalletService) GetERC20TokenBalance(ctx context.Context, client *ethclient.Client, walletAddress string) (*big.Int, error) {
	tokenAddr := common.HexToAddress(bsc.smartContractAddress)
	parsedABI, err := abi.JSON(strings.NewReader(bsc.erc20ABI))
	if err != nil {
		return nil, fmt.Errorf("error parsing ABI: %w", err)
	}

	address := common.HexToAddress(walletAddress)

	// Prepare data for balanceOf call
	data, err := parsedABI.Pack("balanceOf", address)
	if err != nil {
		return nil, fmt.Errorf("error packing data for balanceOf: %w", err)
	}

	// Call the contract
	result, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &tokenAddr,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("error calling token contract: %w", err)
	}

	// Unpack the result
	var tokenBalance *big.Int
	err = parsedABI.UnpackIntoInterface(&tokenBalance, "balanceOf", result)
	if err != nil {
		return nil, fmt.Errorf("error unpacking balanceOf result: %w", err)
	}

	return tokenBalance, nil
}

func (bsc *WalletService) GetGasPrice(ctx context.Context, client *ethclient.Client) (*big.Int, error) {
	return client.SuggestGasPrice(ctx)
}

// TransferFunds transfers USDT from a deposit wallet to a destination wallet
func (bsc *WalletService) TransferFunds(ctx context.Context, client *ethclient.Client, fromWalletID int, toAddress string, amount *big.Int) (string, error) {
	if bsc.masterKey == nil {
		return "", errors.New("master key not initialized")
	}

	// Создаем уникальный ID транзакции для отслеживания в логах
	txID := uuid.New().String()
	startTime := time.Now()

	// Добавляем информацию о транзакции в контекст логирования
	logCtx := context.WithValue(ctx, "tx_id", txID)
	bsc.logger.InfoContext(logCtx, "Starting token transfer",
		"tx_id", txID,
		"from_wallet_id", fromWalletID,
		"to_address", toAddress,
		"amount", amount.String(),
		"status", StatusPending)

	// Get wallet details from database
	wallet, err := bsc.repo.FindWalletByID(ctx, fromWalletID)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to find wallet",
			"tx_id", txID,
			"error", err.Error(),
			"wallet_id", fromWalletID,
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("failed to find wallet with ID %d: %w", fromWalletID, err)
	}

	if wallet == nil {
		bsc.logger.ErrorContext(logCtx, "Wallet not found",
			"tx_id", txID,
			"wallet_id", fromWalletID,
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("wallet with ID %d not found", fromWalletID)
	}

	// Parse derivation path to get the child key index
	derivationPath := wallet.DerivationPath
	bsc.logger.InfoContext(logCtx, "Using derivation path",
		"tx_id", txID,
		"path", derivationPath,
		"wallet", wallet.Address)

	// Extract user ID and index from derivation path
	userID, index, err := ParseDerivationPath(derivationPath)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to parse derivation path",
			"tx_id", txID,
			"error", err.Error(),
			"path", derivationPath,
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", err
	}

	// Get child key and private key
	childKey, err := GetChildKey(bsc.masterKey, userID, index)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to get child key",
			"tx_id", txID,
			"error", err.Error(),
			"user_id", userID,
			"index", index,
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", err
	}

	privateKey, fromAddress, err := GetWalletPrivateKey(childKey)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to get wallet private key",
			"tx_id", txID,
			"error", err.Error(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", err
	}

	// Get the latest nonce for the sender address
	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to get nonce",
			"tx_id", txID,
			"error", err.Error(),
			"address", fromAddress.Hex(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get gas price
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to get gas price",
			"tx_id", txID,
			"error", err.Error(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("failed to get gas price: %w", err)
	}

	// Логируем информацию о газе
	bsc.logger.InfoContext(logCtx, "Got gas price",
		"tx_id", txID,
		"gas_price", gasPrice.String(),
		"nonce", nonce)

	// Create token transfer data
	// USDT contract address on BSC
	tokenAddress := common.HexToAddress(workers.USDTContractAddress)

	// Create ERC20 transfer data
	data := CreateERC20TransferData(toAddress, amount)

	// Estimate gas limit
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  fromAddress,
		To:    &tokenAddress,
		Value: big.NewInt(0),
		Data:  data,
	})
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to estimate gas",
			"tx_id", txID,
			"error", err.Error(),
			"from", fromAddress.Hex(),
			"to", toAddress,
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("failed to estimate gas: %w", err)
	}

	// Add 20% buffer to gas limit
	gasLimit = gasLimit * 12 / 10

	bsc.logger.InfoContext(logCtx, "Estimated gas limit",
		"tx_id", txID,
		"gas_limit", gasLimit,
		"gas_limit_with_buffer", gasLimit)

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
		bsc.logger.ErrorContext(logCtx, "Failed to get chain ID",
			"tx_id", txID,
			"error", err.Error(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Sign the transaction
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to sign transaction",
			"tx_id", txID,
			"error", err.Error(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Calculate the total cost in gas
	gasCost := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))

	// Send the transaction
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to send transaction",
			"tx_id", txID,
			"error", err.Error(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	bsc.logger.InfoContext(logCtx, "Transaction sent successfully",
		"tx_id", txID,
		"tx_hash", txHash,
		"from", fromAddress.Hex(),
		"to", toAddress,
		"amount", amount.String(),
		"gas_price", gasPrice.String(),
		"gas_limit", gasLimit,
		"gas_cost", gasCost.String(),
		"chain_id", chainID.String(),
		"status", StatusSuccess,
		"duration", time.Since(startTime).String())

	return txHash, nil
}

func (bsc *WalletService) TransferAllBNB(ctx context.Context, toAddress, depositUserWalletAddress string, userID, index int) (string, error) {
	// Создаем уникальный ID транзакции для отслеживания в логах
	txID := uuid.New().String()
	startTime := time.Now()

	// Добавляем информацию о транзакции в контекст логирования
	logCtx := context.WithValue(ctx, "tx_id", txID)
	bsc.logger.InfoContext(logCtx, "Starting BNB transfer",
		"tx_id", txID,
		"from_address", depositUserWalletAddress,
		"to_address", toAddress,
		"user_id", userID,
		"index", index,
		"status", StatusPending,
		"operation", "transfer_all_bnb")

	masterKey := CreateMasterKey(bsc.seed)

	// Получаем child key
	childKey, err := GetChildKey(masterKey, int64(userID), int64(index))
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to get child key",
			"tx_id", txID,
			"error", err.Error(),
			"user_id", userID,
			"index", index,
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", err
	}

	// Конвертируем в ECDSA приватный ключ
	privateKey, fromAddress, err := GetWalletPrivateKey(childKey)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to get wallet private key",
			"tx_id", txID,
			"error", err.Error(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", err
	}

	// Проверяем, что адрес соответствует ожидаемому
	expectedAddress := depositUserWalletAddress

	if !strings.EqualFold(fromAddress.Hex(), expectedAddress) {
		bsc.logger.WarnContext(logCtx, "Generated address doesn't match expected",
			"tx_id", txID,
			"generated", fromAddress.Hex(),
			"expected", expectedAddress,
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("cannot derive correct private key for wallet %s, generated %s instead",
			expectedAddress, fromAddress.Hex())
	}

	bsc.logger.InfoContext(logCtx, "Successfully derived private key for wallet",
		"tx_id", txID,
		"address", fromAddress.Hex())

	// Подключаемся к блокчейну
	client, err := GetBSCClient(ctx, bsc.logger)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to connect to blockchain",
			"tx_id", txID,
			"error", err.Error(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", err
	}
	defer client.Close()

	// Получаем nonce для адреса депозитного кошелька пользователя
	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to get nonce",
			"tx_id", txID,
			"error", err.Error(),
			"address", fromAddress.Hex(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("failed to get nonce: %w", err)
	}

	// Получаем текущий баланс
	balance, err := client.BalanceAt(ctx, fromAddress, nil)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to get balance",
			"tx_id", txID,
			"error", err.Error(),
			"address", fromAddress.Hex(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("failed to get balance: %w", err)
	}

	// Логируем текущий баланс
	bsc.logger.InfoContext(logCtx, "Current BNB balance",
		"tx_id", txID,
		"balance_wei", balance.String(),
		"balance_bnb", WeiToEther(balance).Text('f', 18))

	// Проверяем, что есть что отправлять
	if balance.Cmp(big.NewInt(0)) <= 0 {
		bsc.logger.WarnContext(logCtx, "Balance is zero, nothing to transfer",
			"tx_id", txID,
			"address", fromAddress.Hex(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("balance is zero, nothing to transfer")
	}

	// Получаем текущую цену газа
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to get gas price",
			"tx_id", txID,
			"error", err.Error(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("failed to get gas price: %w", err)
	}

	// Стандартный лимит газа для перевода BNB
	gasLimit := uint64(21000)

	// Рассчитываем комиссию за транзакцию
	fee := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))

	// Логируем информацию о газе
	bsc.logger.InfoContext(logCtx, "Gas information",
		"tx_id", txID,
		"gas_price", gasPrice.String(),
		"gas_limit", gasLimit,
		"fee_wei", fee.String(),
		"fee_bnb", WeiToEther(fee).Text('f', 18))

	// Проверяем, что баланс больше комиссии
	if balance.Cmp(fee) <= 0 {
		bsc.logger.WarnContext(logCtx, "Balance is less than transaction fee",
			"tx_id", txID,
			"balance_wei", balance.String(),
			"fee_wei", fee.String(),
			"balance_bnb", WeiToEther(balance).Text('f', 18),
			"fee_bnb", WeiToEther(fee).Text('f', 18),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("balance is less than transaction fee: %s < %s",
			balance.String(), fee.String())
	}

	// Рассчитываем сумму для отправки (баланс - комиссия)
	amount := new(big.Int).Sub(balance, fee)

	bsc.logger.InfoContext(logCtx, "Amount to transfer after fee",
		"tx_id", txID,
		"amount_wei", amount.String(),
		"amount_bnb", WeiToEther(amount).Text('f', 18))

	// Адрес получателя
	to := common.HexToAddress(toAddress)

	// Создаем транзакцию
	tx := types.NewTransaction(nonce, to, amount, gasLimit, gasPrice, nil)

	// Получаем ID цепи
	chainID, err := client.ChainID(ctx)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to get chain ID",
			"tx_id", txID,
			"error", err.Error(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Подписываем транзакцию
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to sign transaction",
			"tx_id", txID,
			"error", err.Error(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Отправляем транзакцию
	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to send transaction",
			"tx_id", txID,
			"error", err.Error(),
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	txHash := signedTx.Hash().Hex()
	bsc.logger.InfoContext(logCtx, "BNB transaction sent successfully",
		"tx_id", txID,
		"tx_hash", txHash,
		"from", fromAddress.Hex(),
		"to", toAddress,
		"amount_wei", amount.String(),
		"amount_bnb", WeiToEther(amount).Text('f', 18),
		"fee_wei", fee.String(),
		"fee_bnb", WeiToEther(fee).Text('f', 18),
		"status", StatusSuccess,
		"duration", time.Since(startTime).String())

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

func CreateMasterKey(seed string) *bip32.Key {
	seedBytes := bip39.NewSeed(seed, "")
	masterKey, err := bip32.NewMasterKey(seedBytes)
	if err != nil {
		log.Fatal("Failed to create master key")
	}

	return masterKey
}

// GetBSCClient connects to one of the BSC RPC endpoints
func GetBSCClient(ctx context.Context, logger *slog.Logger) (*ethclient.Client, error) {
	// Список RPC эндпоинтов BSC (для резервирования)
	bscRpcEndpoints := []string{
		"https://bsc-dataseed.binance.org/",
		"https://bsc-dataseed1.binance.org/",
		"https://bsc-dataseed2.binance.org/",
		"https://bsc-dataseed3.binance.org/",
		"https://bsc-dataseed4.binance.org/",
	}

	// Пробуем подключиться к разным эндпоинтам
	var client *ethclient.Client
	var err error
	var lastErr error

	for _, endpoint := range bscRpcEndpoints {
		logger.Info("Trying to connect to BSC endpoint", "endpoint", endpoint)
		client, err = ethclient.DialContext(ctx, endpoint)
		if err == nil {
			logger.Info("Successfully connected to BSC", "endpoint", endpoint)
			return client, nil
		}
		lastErr = err
		logger.Warn("Failed to connect to BSC endpoint", "endpoint", endpoint, "error", err)
	}

	return nil, fmt.Errorf("failed to connect to any BSC endpoint: %w", lastErr)
}

// GetChildKey generates a child key from the master key based on user ID and index
func GetChildKey(masterKey *bip32.Key, userID, index int64) (*bip32.Key, error) {
	// Create a unique child key based on both user ID and index
	childKeyIndex := uint32(userID*1000 + index)
	childKey, err := masterKey.NewChildKey(childKeyIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to create child key: %w", err)
	}
	return childKey, nil
}

// GetWalletPrivateKey converts a child key to an ECDSA private key
func GetWalletPrivateKey(childKey *bip32.Key) (*ecdsa.PrivateKey, common.Address, error) {
	privateKey, err := crypto.ToECDSA(childKey.Key)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to convert to ECDSA: %w", err)
	}

	// Get the wallet address from the private key
	address := crypto.PubkeyToAddress(privateKey.PublicKey)

	return privateKey, address, nil
}

// WeiToEther converts wei amount to ether (or any token with 18 decimals)
func WeiToEther(wei *big.Int) *big.Float {
	return new(big.Float).Quo(
		new(big.Float).SetInt(wei),
		new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)),
	)
}

// EtherToWei converts ether amount to wei
func EtherToWei(ether *big.Float) *big.Int {
	// Create wei representation (10^18 wei = 1 ether)
	ethWei := new(big.Float).Mul(
		ether,
		new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)),
	)

	// Convert to big.Int
	wei := new(big.Int)
	ethWei.Int(wei)
	return wei
}

// ParseDerivationPath extracts userID and index from derivation path
func ParseDerivationPath(derivationPath string) (int64, int64, error) {
	var userID, index int64

	// m – корневой (master) ключ.
	// 44' – следование стандарту BIP-44 (универсальный путь для multi-account HD-кошельков).
	// 60' – идентификатор монеты. В данном случае 60 – это Ethereum, согласно https://github.com/satoshilabs/slips/blob/master/slip-0044.md
	// %d' – ID аккаунта (в коде он передается в userID), используется для разделения кошельков по пользователям.
	// 0 – chain type, где:
	//	•	0 – внешние (external) адреса (обычно используются для получения средств).
	//	•	1 – внутренние (internal) адреса (обычно для сдачи в транзакциях).
	// %d – индекс адреса в этой цепочке (index), определяет конкретный адрес внутри аккаунта.
	_, err := fmt.Sscanf(derivationPath, "m/44'/60'/%d'/0/%d", &userID, &index)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse derivation path: %w", err)
	}
	return userID, index, nil
}

// CreateERC20TransferData creates transaction data for ERC20 token transfer
func CreateERC20TransferData(toAddress string, amount *big.Int) []byte {
	// Create token transfer data
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

	return data
}
