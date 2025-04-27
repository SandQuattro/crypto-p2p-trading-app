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

	"github.com/sand/crypto-p2p-trading-app/backend/internal/shared"

	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/google/uuid"

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

// Blockchain network configuration
const (
	// Token contract addresses
	MainnetUSDTAddress = "0x55d398326f99059fF775485246999027B3197955" // USDT on BSC Mainnet
	TestnetUSDTAddress = "0x337610d27c682E347C9cD60BD4b3b107C9d34dDd" // USDT on BSC Testnet
)

// GetUSDTContractAddress returns the appropriate USDT contract address based on mode
func GetUSDTContractAddress() string {
	if shared.IsBlockchainDebugMode() {
		return TestnetUSDTAddress
	}
	return MainnetUSDTAddress
}

// Параметры для логирования
const (
	// Статусы операций
	StatusSuccess = "success"
	StatusFailure = "failure"
	StatusPending = "pending"

	// Приоритеты транзакций
	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"

	// Множители для определения gas price в зависимости от приоритета
	// Medium - базовый приоритет (x1.0)
	GasPriceMultiplierLow    = 0.8 // 80% от рекомендуемой цены
	GasPriceMultiplierMedium = 1.0 // 100% от рекомендуемой цены
	GasPriceMultiplierHigh   = 1.3 // 130% от рекомендуемой цены

	// Параметры для ускорения транзакций
	SpeedupGasMultiplier = 1.2              // Множитель для цены газа при ускорении
	MaxPendingTxTime     = 5 * time.Minute  // Максимальное время ожидания транзакции
	SpeedupCheckInterval = 30 * time.Second // Интервал проверки зависших транзакций

	// Параметры мониторинга баланса
	BalanceMonitorInterval        = 5 * time.Minute // Интервал проверки балансов кошельков
	LowBalanceThresholdBNB        = "0.01"          // Порог низкого баланса BNB (в эфирных единицах)
	CriticalBalanceThresholdBNB   = "0.005"         // Критический порог баланса BNB
	LowBalanceThresholdToken      = "10.0"          // Порог низкого баланса токена
	CriticalBalanceThresholdToken = "5.0"           // Критический порог баланса токена
)

// Структура для хранения данных о транзакциях для отслеживания
type PendingTransaction struct {
	TxHash      string
	FromAddress common.Address
	ToAddress   common.Address
	Nonce       uint64
	Amount      *big.Int
	GasPrice    *big.Int
	GasLimit    uint64
	PrivateKey  *ecdsa.PrivateKey
	Data        []byte
	CreatedAt   time.Time
}

type WalletsRepository interface {
	FindWalletByAddress(ctx context.Context, address string) (*entities.Wallet, error)
	FindWalletByID(ctx context.Context, id int) (*entities.Wallet, error)
	IsWalletTracked(ctx context.Context, address string) (bool, error)
	GetAllTrackedWallets(ctx context.Context) ([]entities.Wallet, error)
	GetLastWalletIndexForUser(ctx context.Context, userID int64) (uint32, error)
	TrackWalletWithUserAndIndex(ctx context.Context, address string, derivationPath string, userID int64, index uint32, isTestNet bool) (int, error)
	GetAllTrackedWalletsForUser(ctx context.Context, userID int64) ([]entities.Wallet, error)
}

var _ WalletsRepository = (*repository.WalletsRepository)(nil)

type WalletService struct {
	logger *slog.Logger

	isTestNet bool

	erc20ABI, smartContractAddress string

	seed      string
	masterKey *bip32.Key
	wallets   map[string]bool // In-memory cache of tracked wallets
	walletsMu sync.RWMutex    // Mutex for wallets map

	repo WalletsRepository

	transactions *TransactionServiceImpl
	orderService *OrderService // Добавляем OrderService для доступа к методам работы с заказами

	// Отслеживание и ускорение транзакций
	pendingTxs       map[string]*PendingTransaction       // Карта ожидающих транзакций (ключ - хеш транзакции)
	pendingTxsByAddr map[common.Address]map[uint64]string // Карта адрес -> нонс -> хеш транзакции
	pendingTxsMu     sync.RWMutex                         // Мьютекс для защиты карты транзакций

	// Мониторинг балансов кошельков
	walletBalances   map[string]*entities.WalletBalance // Карта адрес -> информация о балансе
	walletBalancesMu sync.RWMutex                       // Мьютекс для защиты карты балансов

	mu sync.Mutex
}

func NewWalletService(
	logger *slog.Logger,
	seed string,
	transactions *TransactionServiceImpl,
	walletsRepo *repository.WalletsRepository,
	orderService *OrderService, // Добавляем параметр OrderService
) (*WalletService, error) {
	// Get the appropriate USDT contract address based on mode
	contractAddress := GetUSDTContractAddress()

	ws := &WalletService{
		logger: logger,

		erc20ABI:             `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"type":"function"}]`,
		smartContractAddress: contractAddress,

		seed:         seed,
		masterKey:    CreateMasterKey(seed),
		wallets:      make(map[string]bool),
		transactions: transactions,
		repo:         walletsRepo,
		orderService: orderService, // Инициализируем OrderService

		// Инициализация карт для отслеживания транзакций
		pendingTxs:       make(map[string]*PendingTransaction),
		pendingTxsByAddr: make(map[common.Address]map[uint64]string),

		// Мониторинг балансов кошельков
		walletBalances: make(map[string]*entities.WalletBalance),
	}

	// Log which mode we're operating in
	if shared.IsBlockchainDebugMode() {
		ws.isTestNet = true
		logger.Warn("Wallet service initialized in DEBUG mode (using BSC Testnet)")
	} else {
		ws.isTestNet = false
		logger.Warn("Wallet service initialized in PRODUCTION mode (using BSC Mainnet)")
	}

	// Load tracked wallets from database into memory cache
	if err := ws.loadWalletsFromDB(context.Background()); err != nil {
		logger.Error("Failed to load wallets from database", "error", err)
	}

	// Запуск горутины для отслеживания и ускорения зависших транзакций
	go ws.monitorPendingTransactions(context.Background())

	// Запуск горутины для мониторинга балансов кошельков
	go ws.monitorWalletBalances(context.Background())

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
	if walletID, err = bsc.repo.TrackWalletWithUserAndIndex(ctx, address, derivationPath, userID, newIndex, bsc.isTestNet); err != nil {
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
	if _, err = bsc.repo.TrackWalletWithUserAndIndex(ctx, address, derivationPath, userID, newIndex, bsc.isTestNet); err != nil {
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

// GetWalletDetailsExtendedForUser retrieves extended wallet details including creation date
func (bsc *WalletService) GetWalletDetailsExtendedForUser(ctx context.Context, userID int64) ([]entities.WalletDetailExtended, error) {
	wallets, err := bsc.repo.GetAllTrackedWalletsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	var walletDetails []entities.WalletDetailExtended
	for _, wallet := range wallets {
		walletDetails = append(walletDetails, entities.WalletDetailExtended{
			ID:        int64(wallet.ID),
			UserID:    wallet.UserID,
			Address:   wallet.Address,
			IsTestnet: wallet.IsTestnet,
			CreatedAt: wallet.CreatedAt,
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

// GetGasPriceWithPriority возвращает цену газа с учетом приоритета транзакции
func (bsc *WalletService) GetGasPriceWithPriority(ctx context.Context, client *ethclient.Client, priority string) (*big.Int, error) {
	// Получаем базовую цену газа
	baseGasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get suggested gas price: %w", err)
	}

	// Применяем множитель в зависимости от приоритета
	var multiplier float64
	switch priority {
	case PriorityLow:
		multiplier = GasPriceMultiplierLow
	case PriorityHigh:
		multiplier = GasPriceMultiplierHigh
	default: // Medium priority by default
		multiplier = GasPriceMultiplierMedium
	}

	// Рассчитываем adjusted gas price
	// Конвертируем big.Int в float64 для умножения
	baseGasPriceFloat := new(big.Float).SetInt(baseGasPrice)
	adjustedGasPriceFloat := new(big.Float).Mul(baseGasPriceFloat, big.NewFloat(multiplier))

	// Конвертируем обратно в big.Int
	adjustedGasPrice := new(big.Int)
	adjustedGasPriceFloat.Int(adjustedGasPrice)

	return adjustedGasPrice, nil
}

// GetGasPrice returns the suggested gas price (backward compatibility with default medium priority)
func (bsc *WalletService) GetGasPrice(ctx context.Context, client *ethclient.Client) (*big.Int, error) {
	return bsc.GetGasPriceWithPriority(ctx, client, PriorityMedium)
}

// sendTransaction выполняет общие шаги для отправки транзакции и ее отслеживания
func (bsc *WalletService) sendTransaction(
	ctx context.Context,
	client *ethclient.Client,
	privateKey *ecdsa.PrivateKey,
	fromAddress common.Address,
	toAddress common.Address,
	value *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
	data []byte,
	priority string,
) (string, error) {
	txID := uuid.New().String()
	startTime := time.Now()
	logCtx := context.WithValue(ctx, "tx_id", txID)

	// Получаем nonce для отправителя
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

	// Если цена газа не указана явно, получаем ее с учетом приоритета
	if gasPrice == nil {
		gasPrice, err = bsc.GetGasPriceWithPriority(ctx, client, priority)
		if err != nil {
			bsc.logger.ErrorContext(logCtx, "Failed to get gas price",
				"tx_id", txID,
				"error", err.Error(),
				"priority", priority,
				"status", StatusFailure,
				"duration", time.Since(startTime).String())
			return "", fmt.Errorf("failed to get gas price with priority %s: %w", priority, err)
		}
	}

	// Логируем информацию о газе
	bsc.logger.InfoContext(logCtx, "Transaction parameters",
		"tx_id", txID,
		"from", fromAddress.Hex(),
		"to", toAddress.Hex(),
		"value", value.String(),
		"gas_price", gasPrice.String(),
		"gas_limit", gasLimit,
		"nonce", nonce,
		"priority", priority)

	// Создаем транзакцию
	tx := types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)

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

	// Рассчитываем общую стоимость газа
	gasCost := new(big.Int).Mul(gasPrice, big.NewInt(int64(gasLimit)))

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

	// Добавляем транзакцию для отслеживания и возможного ускорения
	bsc.trackTransaction(txHash, fromAddress, toAddress, nonce, value, gasPrice, gasLimit, privateKey, data)

	bsc.logger.InfoContext(logCtx, "Transaction sent successfully",
		"tx_id", txID,
		"tx_hash", txHash,
		"from", fromAddress.Hex(),
		"to", toAddress.Hex(),
		"value", value.String(),
		"gas_price", gasPrice.String(),
		"gas_limit", gasLimit,
		"gas_cost", gasCost.String(),
		"chain_id", chainID.String(),
		"status", StatusSuccess,
		"duration", time.Since(startTime).String())

	return txHash, nil
}

// TransferFunds transfers USDT from a deposit wallet to a destination wallet
func (bsc *WalletService) TransferFunds(ctx context.Context, client *ethclient.Client, fromWalletID int, toAddress string, amount *big.Int) (string, error) {
	return bsc.TransferFundsWithPriority(ctx, client, fromWalletID, toAddress, amount, PriorityMedium)
}

// TransferFundsWithPriority transfers USDT with specified priority level
func (bsc *WalletService) TransferFundsWithPriority(ctx context.Context, client *ethclient.Client, fromWalletID int, toAddress string, amount *big.Int, priority string) (string, error) {
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
		"priority", priority,
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

	// Create token transfer data
	// USDT contract address on BSC
	tokenAddress := common.HexToAddress(GetUSDTContractAddress())

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

	// Получаем цену газа с учетом приоритета
	gasPrice, err := bsc.GetGasPriceWithPriority(ctx, client, priority)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to get gas price with priority",
			"tx_id", txID,
			"error", err.Error(),
			"priority", priority,
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return "", fmt.Errorf("failed to get gas price: %w", err)
	}

	// Send the transaction
	txHash, err := bsc.sendTransaction(ctx, client, privateKey, fromAddress, tokenAddress, big.NewInt(0), gasLimit, gasPrice, data, priority)
	if err != nil {
		return "", err
	}

	// Дополняем лог информацией о сумме токенов
	bsc.logger.InfoContext(logCtx, "Token transfer complete",
		"tx_id", txID,
		"tx_hash", txHash,
		"token_amount", amount.String(),
		"token_address", GetUSDTContractAddress(),
		"status", StatusSuccess,
		"duration", time.Since(startTime).String())

	return txHash, nil
}

func (bsc *WalletService) TransferAllBNB(ctx context.Context, toAddress, depositUserWalletAddress string, userID, index int) (string, error) {
	return bsc.TransferAllBNBWithPriority(ctx, toAddress, depositUserWalletAddress, userID, index, PriorityMedium)
}

func (bsc *WalletService) TransferAllBNBWithPriority(ctx context.Context, toAddress, depositUserWalletAddress string, userID, index int, priority string) (string, error) {
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
		"priority", priority,
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

	// Получаем цену газа с учетом приоритета
	gasPrice, err := bsc.GetGasPriceWithPriority(ctx, client, priority)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to get gas price with priority",
			"tx_id", txID,
			"error", err.Error(),
			"priority", priority,
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

	// Отправляем транзакцию, используя общую логику
	txHash, err := bsc.sendTransaction(ctx, client, privateKey, fromAddress, to, amount, gasLimit, gasPrice, nil, priority)
	if err != nil {
		return "", err
	}

	// Дополняем лог информацией об отправке BNB
	bsc.logger.InfoContext(logCtx, "BNB transfer complete",
		"tx_id", txID,
		"tx_hash", txHash,
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
	// Check if we're in debug/test mode
	debugMode := shared.IsBlockchainDebugMode()

	// Список RPC эндпоинтов BSC (для резервирования)
	var bscRpcEndpoints []string

	if debugMode {
		// Testnet endpoints for debug/test mode
		bscRpcEndpoints = []string{
			"https://data-seed-prebsc-1-s1.binance.org:8545/",
			"https://data-seed-prebsc-2-s1.binance.org:8545/",
			"https://data-seed-prebsc-1-s2.binance.org:8545/",
			"https://data-seed-prebsc-2-s2.binance.org:8545/",
			"https://data-seed-prebsc-1-s3.binance.org:8545/",
		}
		logger.Info("Using BSC Testnet endpoints (DEBUG MODE)")
	} else {
		// Mainnet endpoints for production
		bscRpcEndpoints = []string{
			"https://bsc-dataseed.binance.org/",
			"https://bsc-dataseed1.binance.org/",
			"https://bsc-dataseed2.binance.org/",
			"https://bsc-dataseed3.binance.org/",
			"https://bsc-dataseed4.binance.org/",
		}
		logger.Info("Using BSC Mainnet endpoints (PRODUCTION MODE)")
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

// monitorPendingTransactions запускает периодическую проверку зависших транзакций
func (bsc *WalletService) monitorPendingTransactions(ctx context.Context) {
	ticker := time.NewTicker(SpeedupCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			bsc.logger.Info("Stopping transaction monitoring due to context cancellation")
			return
		case <-ticker.C:
			bsc.checkAndSpeedupPendingTransactions(ctx)
		}
	}
}

// checkAndSpeedupPendingTransactions проверяет все ожидающие транзакции и ускоряет зависшие
func (bsc *WalletService) checkAndSpeedupPendingTransactions(ctx context.Context) {
	// Получаем клиент BSC
	client, err := GetBSCClient(ctx, bsc.logger)
	if err != nil {
		bsc.logger.Error("Failed to get BSC client for transaction monitoring", "error", err)
		return
	}
	defer client.Close()

	// Копируем карту ожидающих транзакций для безопасной итерации
	bsc.pendingTxsMu.RLock()
	pendingTxsCopy := make(map[string]*PendingTransaction)
	for txHash, tx := range bsc.pendingTxs {
		pendingTxsCopy[txHash] = tx
	}
	bsc.pendingTxsMu.RUnlock()

	now := time.Now()
	for txHash, pendingTx := range pendingTxsCopy {
		// Проверяем, не прошло ли слишком много времени с момента отправки
		if now.Sub(pendingTx.CreatedAt) > MaxPendingTxTime {
			// Проверяем статус транзакции
			_, isPending, err := client.TransactionByHash(ctx, common.HexToHash(txHash))
			if err != nil {
				bsc.logger.Warn("Failed to check transaction status", "tx_hash", txHash, "error", err)
				continue
			}

			// Если транзакция все еще в ожидании, ускоряем ее
			if isPending {
				if err := bsc.speedupTransaction(ctx, client, pendingTx); err != nil {
					bsc.logger.Error("Failed to speed up transaction", "tx_hash", txHash, "error", err)
				}
			} else {
				// Транзакция больше не в ожидании, удаляем из отслеживания
				bsc.removePendingTransaction(pendingTx.TxHash, pendingTx.FromAddress, pendingTx.Nonce)
			}
		}
	}
}

// speedupTransaction ускоряет зависшую транзакцию, отправляя новую с тем же нонсом и увеличенной ценой газа
func (bsc *WalletService) speedupTransaction(ctx context.Context, client *ethclient.Client, pendingTx *PendingTransaction) error {
	// Создаем логический контекст для отслеживания
	txID := uuid.New().String()
	startTime := time.Now()
	logCtx := context.WithValue(ctx, "tx_id", txID)

	bsc.logger.InfoContext(logCtx, "Speeding up stuck transaction",
		"tx_id", txID,
		"original_tx_hash", pendingTx.TxHash,
		"from", pendingTx.FromAddress.Hex(),
		"to", pendingTx.ToAddress.Hex(),
		"nonce", pendingTx.Nonce,
		"original_gas_price", pendingTx.GasPrice.String(),
		"status", StatusPending)

	// Увеличиваем цену газа
	newGasPrice := new(big.Int).Mul(pendingTx.GasPrice, big.NewInt(int64(SpeedupGasMultiplier*100)/100))

	// Создаем новую транзакцию с тем же нонсом, но с увеличенной ценой газа
	tx := types.NewTransaction(
		pendingTx.Nonce,
		pendingTx.ToAddress,
		pendingTx.Amount,
		pendingTx.GasLimit,
		newGasPrice,
		pendingTx.Data,
	)

	// Получаем ID сети
	chainID, err := client.ChainID(ctx)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to get chain ID for speedup",
			"tx_id", txID, "error", err, "status", StatusFailure)
		return fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Подписываем транзакцию
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), pendingTx.PrivateKey)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to sign speedup transaction",
			"tx_id", txID, "error", err, "status", StatusFailure)
		return fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Отправляем транзакцию
	if err = client.SendTransaction(ctx, signedTx); err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to send speedup transaction",
			"tx_id", txID, "error", err, "status", StatusFailure)
		return fmt.Errorf("failed to send transaction: %w", err)
	}

	newTxHash := signedTx.Hash().Hex()
	bsc.logger.InfoContext(logCtx, "Successfully sent speedup transaction",
		"tx_id", txID,
		"new_tx_hash", newTxHash,
		"original_tx_hash", pendingTx.TxHash,
		"from", pendingTx.FromAddress.Hex(),
		"to", pendingTx.ToAddress.Hex(),
		"nonce", pendingTx.Nonce,
		"original_gas_price", pendingTx.GasPrice.String(),
		"new_gas_price", newGasPrice.String(),
		"status", StatusSuccess,
		"duration", time.Since(startTime).String())

	// Обновляем информацию о транзакции в хранилище
	bsc.trackTransaction(newTxHash, pendingTx.FromAddress, pendingTx.ToAddress, pendingTx.Nonce,
		pendingTx.Amount, newGasPrice, pendingTx.GasLimit, pendingTx.PrivateKey, pendingTx.Data)

	// Удаляем старую транзакцию из отслеживания (прямо передаем txHash)
	bsc.removePendingTransaction(pendingTx.TxHash, pendingTx.FromAddress, pendingTx.Nonce)

	return nil
}

// trackTransaction добавляет транзакцию в список ожидающих для возможного ускорения
func (bsc *WalletService) trackTransaction(txHash string, fromAddr, toAddr common.Address, nonce uint64,
	amount, gasPrice *big.Int, gasLimit uint64, privKey *ecdsa.PrivateKey, data []byte) {

	tx := &PendingTransaction{
		TxHash:      txHash,
		FromAddress: fromAddr,
		ToAddress:   toAddr,
		Nonce:       nonce,
		Amount:      amount,
		GasPrice:    gasPrice,
		GasLimit:    gasLimit,
		PrivateKey:  privKey,
		Data:        data,
		CreatedAt:   time.Now(),
	}

	bsc.pendingTxsMu.Lock()
	defer bsc.pendingTxsMu.Unlock()

	// Сохраняем транзакцию в карте по хешу
	bsc.pendingTxs[txHash] = tx

	// Инициализируем карту нонсов для адреса, если она не существует
	if _, exists := bsc.pendingTxsByAddr[fromAddr]; !exists {
		bsc.pendingTxsByAddr[fromAddr] = make(map[uint64]string)
	}

	// Сохраняем связь адрес -> нонс -> хеш транзакции
	bsc.pendingTxsByAddr[fromAddr][nonce] = txHash
}

// removePendingTransaction удаляет транзакцию из списка ожидающих
func (bsc *WalletService) removePendingTransaction(txHash string, fromAddr common.Address, nonce uint64) {
	bsc.pendingTxsMu.Lock()
	defer bsc.pendingTxsMu.Unlock()

	// Удаляем из карты по хешу
	delete(bsc.pendingTxs, txHash)

	// Удаляем связь адрес -> нонс -> хеш, если она существует
	if nonceMap, exists := bsc.pendingTxsByAddr[fromAddr]; exists {
		// Проверяем, что хеш по этому нонсу соответствует удаляемому
		if storedHash, exists := nonceMap[nonce]; exists && storedHash == txHash {
			delete(nonceMap, nonce)
		}

		// Если карта нонсов пуста, удаляем и ее
		if len(nonceMap) == 0 {
			delete(bsc.pendingTxsByAddr, fromAddr)
		}
	}
}

// CheckBalance retrieves the USDT balance for the given wallet address
func (bsc *WalletService) CheckBalance(ctx context.Context, client *ethclient.Client, walletAddress string) (*big.Int, error) {
	// Create a logger context for tracking
	txID := uuid.New().String()
	startTime := time.Now()
	logCtx := context.WithValue(ctx, "tx_id", txID)

	bsc.logger.InfoContext(logCtx, "Checking wallet balance",
		"tx_id", txID,
		"address", walletAddress,
		"status", StatusPending)

	// Get the token balance using the existing method
	balance, err := bsc.GetERC20TokenBalance(ctx, client, walletAddress)
	if err != nil {
		bsc.logger.ErrorContext(logCtx, "Failed to get token balance",
			"tx_id", txID,
			"error", err.Error(),
			"address", walletAddress,
			"status", StatusFailure,
			"duration", time.Since(startTime).String())
		return nil, fmt.Errorf("failed to get token balance: %w", err)
	}

	// Log success
	bsc.logger.InfoContext(logCtx, "Successfully retrieved token balance",
		"tx_id", txID,
		"address", walletAddress,
		"balance", balance.String(),
		"status", StatusSuccess,
		"duration", time.Since(startTime).String())

	return balance, nil
}

// GenerateSeedPhrase creates a new random BIP39 mnemonic seed phrase
// The entropy parameter specifies the strength:
// - 128 bits = 12 words
// - 160 bits = 15 words
// - 192 bits = 18 words
// - 224 bits = 21 words
// - 256 bits = 24 words
func (bsc *WalletService) GenerateSeedPhrase(entropyBits int) (string, error) {
	// Validate entropy bits
	if entropyBits != 128 && entropyBits != 160 && entropyBits != 192 && entropyBits != 224 && entropyBits != 256 {
		return "", fmt.Errorf("invalid entropy bits: %d (must be 128, 160, 192, 224, or 256)", entropyBits)
	}

	// Generate entropy
	entropy, err := bip39.NewEntropy(entropyBits)
	if err != nil {
		return "", fmt.Errorf("failed to generate entropy: %w", err)
	}

	// Convert entropy to mnemonic
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("failed to generate mnemonic: %w", err)
	}

	return mnemonic, nil
}

// monitorWalletBalances запускает периодическую проверку балансов кошельков
func (bsc *WalletService) monitorWalletBalances(ctx context.Context) {
	ticker := time.NewTicker(BalanceMonitorInterval)
	defer ticker.Stop()

	bsc.logger.Info("Starting wallet balance monitoring",
		"interval", BalanceMonitorInterval.String())

	// Выполняем первоначальную проверку балансов
	if err := bsc.checkAllWalletBalances(ctx); err != nil {
		bsc.logger.Error("Failed to perform initial wallet balance check", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			bsc.logger.Info("Wallet balance monitoring stopped")
			return
		case <-ticker.C:
			if err := bsc.checkAllWalletBalances(ctx); err != nil {
				bsc.logger.Error("Failed to check wallet balances", "error", err)
			}
		}
	}
}

// checkAllWalletBalances проверяет балансы всех отслеживаемых кошельков
func (bsc *WalletService) checkAllWalletBalances(ctx context.Context) error {
	// Создаем клиент для запросов к блокчейну
	client, err := GetBSCClient(ctx, bsc.logger)
	if err != nil {
		return fmt.Errorf("failed to create BSC client: %w", err)
	}
	defer client.Close()

	// Получаем все отслеживаемые кошельки
	wallets, err := bsc.repo.GetAllTrackedWallets(ctx)
	if err != nil {
		return fmt.Errorf("failed to get tracked wallets: %w", err)
	}

	// Преобразуем пороги в big.Int для сравнения
	lowBNBThreshold, _ := new(big.Float).SetString(LowBalanceThresholdBNB)
	criticalBNBThreshold, _ := new(big.Float).SetString(CriticalBalanceThresholdBNB)
	lowTokenThreshold, _ := new(big.Float).SetString(LowBalanceThresholdToken)
	criticalTokenThreshold, _ := new(big.Float).SetString(CriticalBalanceThresholdToken)

	// Преобразуем в Wei
	lowBNBThresholdWei := EtherToWei(lowBNBThreshold)
	criticalBNBThresholdWei := EtherToWei(criticalBNBThreshold)
	lowTokenThresholdWei := EtherToWei(lowTokenThreshold)
	criticalTokenThresholdWei := EtherToWei(criticalTokenThreshold)

	// Проверяем баланс каждого кошелька
	for _, wallet := range wallets {
		address := wallet.Address
		walletAddress := common.HexToAddress(address)

		// Получаем баланс BNB
		bnbBalance, err := client.BalanceAt(ctx, walletAddress, nil)
		if err != nil {
			bsc.logger.ErrorContext(ctx, "Failed to get BNB balance",
				"address", address,
				"error", err)
			continue
		}

		// Получаем баланс токена (USDT)
		tokenBalance, err := bsc.GetERC20TokenBalance(ctx, client, address)
		if err != nil {
			bsc.logger.ErrorContext(ctx, "Failed to get token balance",
				"address", address,
				"token", bsc.smartContractAddress,
				"error", err)
			// Продолжаем, даже если не смогли получить баланс токена
			tokenBalance = big.NewInt(0)
		}

		// Определяем статус баланса
		var status entities.BalanceStatus
		if bnbBalance.Cmp(criticalBNBThresholdWei) <= 0 ||
			tokenBalance.Cmp(criticalTokenThresholdWei) <= 0 {
			status = entities.BalanceStatusCritical
		} else if bnbBalance.Cmp(lowBNBThresholdWei) <= 0 ||
			tokenBalance.Cmp(lowTokenThresholdWei) <= 0 {
			status = entities.BalanceStatusLow
		} else {
			status = entities.BalanceStatusOK
		}

		// Сохраняем информацию о балансе
		walletBalance := &entities.WalletBalance{
			Address:       address,
			TokenBalance:  tokenBalance,
			NativeBalance: bnbBalance,
			Status:        status,
			LastChecked:   time.Now(),
		}

		// Проверяем, изменился ли статус баланса
		bsc.walletBalancesMu.Lock()
		prevBalance, exists := bsc.walletBalances[address]
		bsc.walletBalances[address] = walletBalance
		bsc.walletBalancesMu.Unlock()

		// Логируем информацию о балансе
		bnbFloat := WeiToEther(bnbBalance)
		tokenFloat := WeiToEther(tokenBalance)

		// Логируем информацию только при изменении статуса или первой проверке
		if !exists || prevBalance.Status != status {
			logLevel := slog.LevelInfo
			if status == entities.BalanceStatusCritical {
				logLevel = slog.LevelWarn
			} else if status == entities.BalanceStatusLow {
				logLevel = slog.LevelInfo
			}

			bsc.logger.Log(ctx, logLevel, "Wallet balance status",
				"address", address,
				"bnb_balance", bnbFloat.Text('f', 18),
				"token_balance", tokenFloat.Text('f', 18),
				"status", status,
				"user_id", wallet.UserID)
		} else {
			// Для отладки, логируем на уровне Debug при отсутствии изменений
			bsc.logger.DebugContext(ctx, "Wallet balance checked",
				"address", address,
				"bnb_balance", bnbFloat.Text('f', 18),
				"token_balance", tokenFloat.Text('f', 18),
				"status", status)
		}
	}

	return nil
}

// GetUserWalletsBalances возвращает информацию о балансах кошельков для указанного пользователя из кеша.
// Важно: подразумевается, что monitorWalletBalances регулярно обновляет кеш bsc.walletBalances.
func (bsc *WalletService) GetUserWalletsBalances(ctx context.Context, userID int) (map[string]*entities.WalletBalance, error) {
	bsc.logger.DebugContext(ctx, "Fetching wallet balances for user", "user_id", userID)

	// Получить все кошельки для данного пользователя из репозитория
	userWallets, err := bsc.repo.GetAllTrackedWalletsForUser(ctx, int64(userID))
	if err != nil {
		bsc.logger.ErrorContext(ctx, "Failed to get tracked wallets for user", "error", err, "user_id", userID)
		return nil, fmt.Errorf("failed to get wallets for user %d: %w", userID, err)
	}

	if len(userWallets) == 0 {
		bsc.logger.InfoContext(ctx, "No wallets found for user", "user_id", userID)
		return make(map[string]*entities.WalletBalance), nil
	}

	// 2. Создать карту для результатов
	userBalances := make(map[string]*entities.WalletBalance)

	// 3. Прочитать актуальные балансы из кеша (безопасно)
	bsc.walletBalancesMu.RLock()
	defer bsc.walletBalancesMu.RUnlock()

	// 4. Отфильтровать балансы, оставив только те, что принадлежат пользователю
	for _, wallet := range userWallets {
		address := wallet.Address
		if balance, ok := bsc.walletBalances[address]; ok {
			// Копируем баланс, чтобы избежать гонки данных при возврате указателя
			userBalances[address] = &entities.WalletBalance{
				Address:       balance.Address,
				TokenBalance:  new(big.Int).Set(balance.TokenBalance),  // Глубокое копирование
				NativeBalance: new(big.Int).Set(balance.NativeBalance), // Глубокое копирование
				Status:        balance.Status,
				LastChecked:   balance.LastChecked,
			}
		} else {
			// Логируем, если баланс для отслеживаемого кошелька пользователя отсутствует в кеше
			// Это может указывать на задержку в monitorWalletBalances или другую проблему
			bsc.logger.WarnContext(ctx, "Balance not found in cache for user's tracked wallet", "address", address, "user_id", userID)
			// Можно инициализировать с нулевым балансом или пропустить
			// userBalances[address] = &entities.WalletBalance{ Address: address, TokenBalance: big.NewInt(0), NativeBalance: big.NewInt(0), Status: entities.BalanceStatusUnknown }
		}
	}

	bsc.logger.DebugContext(ctx, "Returning balances for user", "user_id", userID, "count", len(userBalances))
	return userBalances, nil
}

// GetWalletBalances возвращает информацию о балансах всех отслеживаемых кошельков
func (bsc *WalletService) GetWalletBalances(ctx context.Context) (map[string]*entities.WalletBalance, error) {
	// Обновляем балансы перед возвратом
	if err := bsc.checkAllWalletBalances(ctx); err != nil {
		return nil, fmt.Errorf("failed to update wallet balances: %w", err)
	}

	// Копируем карту балансов для возврата
	bsc.walletBalancesMu.RLock()
	defer bsc.walletBalancesMu.RUnlock()

	balances := make(map[string]*entities.WalletBalance, len(bsc.walletBalances))
	for addr, balance := range bsc.walletBalances {
		balances[addr] = &entities.WalletBalance{
			Address:       balance.Address,
			TokenBalance:  new(big.Int).Set(balance.TokenBalance),
			NativeBalance: new(big.Int).Set(balance.NativeBalance),
			Status:        balance.Status,
			LastChecked:   balance.LastChecked,
		}
	}

	return balances, nil
}

// GetWalletBalance возвращает информацию о балансе конкретного кошелька
func (bsc *WalletService) GetWalletBalance(ctx context.Context, address string) (*entities.WalletBalance, error) {
	// Проверяем, отслеживается ли этот кошелек
	tracked, err := bsc.IsOurWallet(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to check if wallet is tracked: %w", err)
	}
	if !tracked {
		return nil, fmt.Errorf("wallet %s is not tracked", address)
	}

	// Создаем клиент для запросов к блокчейну
	client, err := GetBSCClient(ctx, bsc.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create BSC client: %w", err)
	}
	defer client.Close()

	walletAddress := common.HexToAddress(address)

	// Получаем баланс BNB
	bnbBalance, err := client.BalanceAt(ctx, walletAddress, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get BNB balance: %w", err)
	}

	// Получаем баланс токена (USDT)
	tokenBalance, err := bsc.GetERC20TokenBalance(ctx, client, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get token balance: %w", err)
	}

	// Преобразуем пороги в big.Int для сравнения
	lowBNBThreshold, _ := new(big.Float).SetString(LowBalanceThresholdBNB)
	criticalBNBThreshold, _ := new(big.Float).SetString(CriticalBalanceThresholdBNB)
	lowTokenThreshold, _ := new(big.Float).SetString(LowBalanceThresholdToken)
	criticalTokenThreshold, _ := new(big.Float).SetString(CriticalBalanceThresholdToken)

	// Преобразуем в Wei
	lowBNBThresholdWei := EtherToWei(lowBNBThreshold)
	criticalBNBThresholdWei := EtherToWei(criticalBNBThreshold)
	lowTokenThresholdWei := EtherToWei(lowTokenThreshold)
	criticalTokenThresholdWei := EtherToWei(criticalTokenThreshold)

	// Определяем статус баланса
	var status entities.BalanceStatus
	if bnbBalance.Cmp(criticalBNBThresholdWei) <= 0 ||
		tokenBalance.Cmp(criticalTokenThresholdWei) <= 0 {
		status = entities.BalanceStatusCritical
	} else if bnbBalance.Cmp(lowBNBThresholdWei) <= 0 ||
		tokenBalance.Cmp(lowTokenThresholdWei) <= 0 {
		status = entities.BalanceStatusLow
	} else {
		status = entities.BalanceStatusOK
	}

	// Создаем и возвращаем объект с информацией о балансе
	walletBalance := &entities.WalletBalance{
		Address:       address,
		TokenBalance:  tokenBalance,
		NativeBalance: bnbBalance,
		Status:        status,
		LastChecked:   time.Now(),
	}

	// Обновляем кэш балансов
	bsc.walletBalancesMu.Lock()
	bsc.walletBalances[address] = walletBalance
	bsc.walletBalancesMu.Unlock()

	return walletBalance, nil
}

// GetOrderIdForWallet делегирует вызов методу OrderService для получения ID заказа по адресу кошелька
func (bsc *WalletService) GetOrderIdForWallet(ctx context.Context, walletAddress string) (int, error) {
	if bsc.orderService == nil {
		return 0, errors.New("order service not initialized")
	}
	return bsc.orderService.GetOrderIdForWallet(ctx, walletAddress)
}
