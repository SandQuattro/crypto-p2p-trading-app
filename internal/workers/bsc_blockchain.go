package workers

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/shared"

	"github.com/google/uuid"
	"github.com/sand/crypto-p2p-trading-app/backend/config"
	amlEntities "github.com/sand/crypto-p2p-trading-app/backend/internal/aml/entities"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

// LogFields определяет стандартизированные поля для логирования
type LogFields struct {
	// Идентификаторы
	TxID        string `json:"tx_id"`        // Уникальный ID для отслеживания транзакции в системе
	TxHash      string `json:"tx_hash"`      // Хеш транзакции в блокчейне
	BlockNumber uint64 `json:"block_number"` // Номер блока
	BlockHash   string `json:"block_hash"`   // Хеш блока

	// Адреса
	From     string `json:"from"`     // Адрес отправителя
	To       string `json:"to"`       // Адрес получателя
	Contract string `json:"contract"` // Адрес контракта (если применимо)

	// Значения
	Amount    string `json:"amount"`     // Сумма транзакции
	AmountWei string `json:"amount_wei"` // Сумма в wei
	GasUsed   uint64 `json:"gas_used"`   // Использованный газ
	GasPrice  string `json:"gas_price"`  // Цена газа
	GasLimit  uint64 `json:"gas_limit"`  // Лимит газа
	Fee       string `json:"fee"`        // Комиссия за транзакцию

	// Статусы и ошибки
	Status        string `json:"status"`        // Статус транзакции (pending, confirmed, failed)
	Error         string `json:"error"`         // Текст ошибки (если есть)
	Confirmations int64  `json:"confirmations"` // Количество подтверждений

	// Время
	Timestamp time.Time `json:"timestamp"` // Время операции
	Duration  string    `json:"duration"`  // Длительность операции
}

// Стандартизированные статусы транзакций
const (
	TxStatusPending   = "pending"
	TxStatusConfirmed = "confirmed"
	TxStatusFailed    = "failed"
)

// BSC WebSocket endpoints
func GetBSCWebSocketEndpoints() []string {
	if shared.IsBlockchainDebugMode() {
		// Testnet WebSocket endpoints for debug/test mode
		return []string{
			"wss://bsc-testnet.publicnode.com",
			"wss://bsc-testnet.nodereal.io/ws",
			"wss://data-seed-prebsc-1-s1.binance.org:8545/ws",
			"wss://data-seed-prebsc-2-s1.binance.org:8545/ws",
			"wss://data-seed-prebsc-1-s2.binance.org:8545/ws",
		}
	}
	// Mainnet WebSocket endpoints for production
	return []string{
		"wss://bsc-ws-node.nariox.org:443",
		"wss://bsc.getblock.io/mainnet/",
		"wss://bsc-mainnet.nodereal.io/ws",
		"wss://rpc.ankr.com/bsc/ws",
		"wss://bsc.publicnode.com",
	}
}

// BSC HTTP endpoints
var bscHTTPEndpoints = []string{
	"https://bsc-dataseed.binance.org/",
	"https://bsc-dataseed1.binance.org/",
	"https://bsc-dataseed2.binance.org/",
	"https://bsc-dataseed3.binance.org/",
	"https://bsc-dataseed4.binance.org/",
}

type TransactionService interface {
	GetTransactionsByWallet(ctx context.Context, walletAddress string) ([]entities.Transaction, error)
	RecordTransaction(ctx context.Context, txHash common.Hash, walletAddress string, amount *big.Int, blockNumber int64) error
	ConfirmTransaction(ctx context.Context, txHash string) error
	ProcessPendingTransactions(ctx context.Context) error
	MarkTransactionAMLFlagged(ctx context.Context, txHash string) error
}

// AMLService определяет интерфейс для AML проверок
type AMLService interface {
	CheckTransaction(ctx context.Context, txHash common.Hash, sourceAddress, destinationAddress string, amount *big.Int) (*amlEntities.AMLCheckResult, error)
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

	// Методы мониторинга балансов
	GetWalletBalances(ctx context.Context) (map[string]*entities.WalletBalance, error)
	GetUserWalletsBalances(ctx context.Context, userID int) (map[string]*entities.WalletBalance, error)
	GetWalletBalance(ctx context.Context, address string) (*entities.WalletBalance, error)
}

// OrderService defines the interface for order operations.
type OrderService interface {
	MarkOrderForAMLReview(ctx context.Context, orderID int, notes string) error
	RemoveOldOrders(ctx context.Context, olderThan time.Duration) (int64, error)
	GetUserOrders(ctx context.Context, userID int) ([]entities.Order, error)
	CreateOrder(ctx context.Context, userID, walletID int, amount string) error
}

const (
	subscriptionRetryDelay = 10 * time.Second // Delay before retrying subscription
	maxConcurrentChecks    = 100              // Максимальное количество одновременных проверок подтверждений

	// Block fetching retry configuration
	maxBlockFetchRetries = 5                // Maximum number of retries for block fetching
	initialRetryDelay    = 1 * time.Second  // Initial delay before retry
	maxRetryDelay        = 10 * time.Second // Maximum delay between retries
)

// GetContractAddress returns the appropriate contract address based on mode
func GetContractAddress() string {
	if shared.IsBlockchainDebugMode() {
		return "0x337610d27c682E347C9cD60BD4b3b107C9d34dDd" // USDT on BSC Testnet
	}
	return "0x55d398326f99059fF775485246999027B3197955" // USDT on BSC Mainnet
}

// USDTContractAddress returns the appropriate USDT contract address based on mode
// This variable is kept for backward compatibility
var USDTContractAddress = GetContractAddress()

// Define the ERC-20 transfer method signature.
var (
	transferSig = []byte{0xa9, 0x05, 0x9c, 0xbb} // keccak256("transfer(address,uint256)")[0:4]
)

type BinanceSmartChain struct {
	logger *slog.Logger
	config *config.Config

	transactions TransactionService
	wallets      WalletService
	amlService   AMLService   // Добавляем сервис AML проверок
	orders       OrderService // Добавляем сервис ордеров

	// Семафор для ограничения одновременных проверок подтверждений
	confirmationSemaphore chan struct{}

	// Мьютекс для защиты lastProcessedBlock
	mu                 sync.Mutex
	lastProcessedBlock uint64
}

func NewBinanceSmartChain(
	logger *slog.Logger,
	config *config.Config,
	transactions TransactionService,
	wallets WalletService,
	amlService AMLService,
	orders OrderService,
) *BinanceSmartChain {
	// Refresh the USDTContractAddress to ensure it's set correctly based on current environment
	USDTContractAddress = GetContractAddress()

	if shared.IsBlockchainDebugMode() {
		logger.Info("Initializing BSC blockchain monitoring in DEBUG mode (BSC Testnet)",
			"contract_address", USDTContractAddress)
	} else {
		logger.Info("Initializing BSC blockchain monitoring in PRODUCTION mode (BSC Mainnet)",
			"contract_address", USDTContractAddress)
	}

	return &BinanceSmartChain{
		logger:                logger,
		config:                config,
		transactions:          transactions,
		wallets:               wallets,
		amlService:            amlService,
		orders:                orders,
		confirmationSemaphore: make(chan struct{}, maxConcurrentChecks),
	}
}

// SubscribeToTransactions monitors incoming transactions via Web3.
// The service will use WebSocket to listen for new blocks and process incoming transactions.
func (bsc *BinanceSmartChain) SubscribeToTransactions(ctx context.Context, rpcURL string) {
	for {
		bsc.logger.InfoContext(ctx, "Starting blockchain monitoring via WebSocket...")

		// Пытаемся использовать WebSocket подписку
		if err := bsc.subscribeViaWebsocket(ctx); err != nil {
			bsc.logger.ErrorContext(ctx, "WebSocket subscription failed, retrying...",
				"delay", subscriptionRetryDelay, "error", err)

			select {
			case <-ctx.Done():
				return
			case <-time.After(subscriptionRetryDelay):
				continue
			}
		}

		return // If we get here without error, we're done
	}
}

// subscribeViaWebsocket subscribes to new block headers via WebSocket
func (bsc *BinanceSmartChain) subscribeViaWebsocket(ctx context.Context) error {
	// Пробуем все WebSocket эндпоинты до успешного подключения
	var wsClient *ethclient.Client
	var rpcClient *rpc.Client
	var wsEndpoint string
	var err error

	bsc.logger.InfoContext(ctx, "Attempting to connect via WebSocket")

	for _, endpoint := range GetBSCWebSocketEndpoints() {
		bsc.logger.InfoContext(ctx, "Trying WebSocket endpoint", "endpoint", endpoint)

		// Создаем RPC клиент с WebSocket соединением
		rpcClient, err = rpc.DialContext(ctx, endpoint)
		if err != nil {
			bsc.logger.WarnContext(ctx, "Failed to connect to WebSocket endpoint",
				"endpoint", endpoint, "error", err)
			continue
		}

		// Создаем Ethereum клиент на основе RPC клиента
		wsClient = ethclient.NewClient(rpcClient)
		wsEndpoint = endpoint
		bsc.logger.InfoContext(ctx, "Successfully connected to WebSocket endpoint",
			"endpoint", endpoint)
		break
	}

	if wsClient == nil {
		return fmt.Errorf("failed to connect to any WebSocket endpoint")
	}

	defer wsClient.Close()

	// Получаем текущий номер блока для начала мониторинга
	currentBlock, err := wsClient.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current block number: %w", err)
	}

	bsc.mu.Lock()
	bsc.lastProcessedBlock = currentBlock
	bsc.mu.Unlock()

	bsc.logger.InfoContext(ctx, "Starting WebSocket monitoring from block",
		"block", currentBlock, "endpoint", wsEndpoint)

	// Создаем канал для получения заголовков новых блоков
	headers := make(chan *types.Header)

	// Подписываемся на новые заголовки блоков
	subscription, err := wsClient.SubscribeNewHead(ctx, headers)
	if err != nil {
		return fmt.Errorf("failed to subscribe to new headers: %w", err)
	}
	defer subscription.Unsubscribe()

	// Обрабатываем поступающие транзакции каждую минуту
	processTicker := time.NewTicker(1 * time.Minute)
	defer processTicker.Stop()

	// Создаем HTTP клиент для получения полных данных блоков
	// (WebSocket не всегда возвращает полную информацию о блоке)
	httpClient, err := getHTTPClient(ctx, bsc.logger)
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}
	defer httpClient.Close()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("WebSocket subscription done with %w", ctx.Err())

		case err := <-subscription.Err():
			return fmt.Errorf("WebSocket subscription error: %w", err)

		case header := <-headers:
			// Получаем номер блока из заголовка
			blockNumber := header.Number.Uint64()

			bsc.mu.Lock()
			lastProcessed := bsc.lastProcessedBlock
			bsc.mu.Unlock()

			// Проверяем, не пропустили ли мы блоки
			if blockNumber > lastProcessed+1 {
				bsc.logger.WarnContext(ctx, "Missed blocks detected, fetching missing blocks",
					"from", lastProcessed+1, "to", blockNumber-1)

				// Получаем пропущенные блоки через HTTP клиент
				for missedBlock := lastProcessed + 1; missedBlock < blockNumber; missedBlock++ {
					bsc.processBlockByNumber(ctx, httpClient, missedBlock)
				}
			}

			// Обрабатываем текущий блок
			if err := bsc.processBlockHeader(ctx, httpClient, header); err != nil {
				bsc.logger.ErrorContext(ctx, "Failed to process block header",
					"block", blockNumber, "error", err)
			}

			// Обновляем последний обработанный блок
			bsc.mu.Lock()
			if blockNumber > bsc.lastProcessedBlock {
				bsc.lastProcessedBlock = blockNumber
			}
			bsc.mu.Unlock()

		case <-processTicker.C:
			// Периодически обрабатываем ожидающие транзакции
			if err := bsc.transactions.ProcessPendingTransactions(ctx); err != nil {
				bsc.logger.ErrorContext(ctx, "Failed to process pending transactions",
					"error", err)
			}
		}
	}
}

// processBlockByNumber обрабатывает блок по его номеру
func (bsc *BinanceSmartChain) processBlockByNumber(ctx context.Context, client *ethclient.Client, blockNumber uint64) {
	// Добавляем механизм повторных попыток для случаев, когда блок еще не доступен
	maxRetries := 3
	retryDelay := 500 * time.Millisecond

	var block *types.Block
	var err error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		block, err = client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
		if err == nil {
			break // Блок успешно получен
		}

		// Проверяем, является ли ошибка "not found"
		if strings.Contains(err.Error(), "not found") {
			if attempt < maxRetries {
				bsc.logger.InfoContext(ctx, "Block not available yet, retrying",
					"block", blockNumber, "attempt", attempt, "max_retries", maxRetries,
					"retry_delay", retryDelay)

				// Ждем перед следующей попыткой
				select {
				case <-ctx.Done():
					return
				case <-time.After(retryDelay):
					// Увеличиваем задержку для каждой следующей попытки
					retryDelay = retryDelay * 2
					continue
				}
			}
		}

		// Если это не ошибка "not found" или все попытки исчерпаны, логируем ошибку
		bsc.logger.ErrorContext(ctx, "Failed to get block by number",
			"block", blockNumber, "error", err, "attempts", attempt)
		return
	}

	bsc.processBlock(ctx, client, block.Header())
}

// processBlockHeader обрабатывает заголовок блока
func (bsc *BinanceSmartChain) processBlockHeader(ctx context.Context, client *ethclient.Client, header *types.Header) error {
	// Добавляем механизм повторных попыток для случаев, когда блок еще не доступен
	maxRetries := maxBlockFetchRetries
	retryDelay := initialRetryDelay
	startTime := time.Now() // Добавляем измерение времени
	blockNumber := header.Number.Uint64()

	// Fallback clients to use when primary client fails
	var fallbackClient *ethclient.Client
	var fallbackEndpoint string

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// On the third attempt, try with a different RPC endpoint
		currentClient := client
		if attempt == 3 && fallbackClient == nil {
			// Choose a fallback endpoint based on debug mode
			if shared.IsBlockchainDebugMode() {
				// Testnet fallback endpoint
				fallbackEndpoint = "https://data-seed-prebsc-2-s3.binance.org:8545"
			} else {
				// Mainnet fallback endpoint
				fallbackEndpoint = "https://bsc-dataseed2.binance.org"
			}

			var err error
			fallbackClient, err = ethclient.Dial(fallbackEndpoint)
			if err != nil {
				bsc.logger.WarnContext(ctx, "Failed to create fallback client",
					"endpoint", fallbackEndpoint,
					"error", err)
			} else {
				bsc.logger.InfoContext(ctx, "Created fallback client",
					"endpoint", fallbackEndpoint)
				currentClient = fallbackClient
			}
		} else if attempt > 3 && fallbackClient != nil {
			currentClient = fallbackClient
		}

		// Получаем полные данные блока по хешу заголовка
		block, err := currentClient.BlockByHash(ctx, header.Hash())
		if err == nil {
			// If successful with fallback client, log it
			if currentClient != client {
				bsc.logger.InfoContext(ctx, "Successfully retrieved block with fallback client",
					"endpoint", fallbackEndpoint,
					"block_number", blockNumber,
					"attempt", attempt)
			}
			// Блок успешно получен, обрабатываем его
			return bsc.processBlock(ctx, currentClient, block.Header())
		}

		// Проверяем, является ли ошибка "not found"
		if strings.Contains(err.Error(), "not found") {
			// Try alternative method - get block by number as fallback
			if attempt == 2 || attempt == 4 { // On 2nd and 4th attempts, try by number instead
				bsc.logger.InfoContext(ctx, "Trying to get block by number instead of hash",
					"block_number", blockNumber,
					"block_hash", header.Hash().Hex(),
					"attempt", attempt)

				blockByNumber, errByNumber := currentClient.BlockByNumber(ctx, header.Number)
				if errByNumber == nil {
					// Block successfully retrieved by number
					bsc.logger.InfoContext(ctx, "Successfully retrieved block by number",
						"block_number", blockNumber,
						"duration", time.Since(startTime).String())
					return bsc.processBlock(ctx, currentClient, blockByNumber.Header())
				}

				bsc.logger.WarnContext(ctx, "Failed to get block by number too",
					"block_number", blockNumber,
					"error", errByNumber)
			}

			if attempt < maxRetries {
				bsc.logger.InfoContext(ctx, "Block not available yet, retrying",
					"block_hash", header.Hash().Hex(),
					"block", blockNumber,
					"attempt", attempt,
					"max_retries", maxRetries,
					"retry_delay", retryDelay)

				// Ждем перед следующей попыткой
				select {
				case <-ctx.Done():
					return fmt.Errorf("context done: %w", ctx.Err())
				case <-time.After(retryDelay):
					// Увеличиваем задержку для каждой следующей попытки
					retryDelay = retryDelay * 2
					// Ensure we don't exceed maximum delay
					if retryDelay > maxRetryDelay {
						retryDelay = maxRetryDelay
					}
					continue
				}
			}
		}

		// Если это не ошибка "not found" или все попытки исчерпаны, возвращаем ошибку
		bsc.logger.ErrorContext(ctx, "Failed to get block",
			"error", err,
			"block_hash", header.Hash().Hex(),
			"block", blockNumber,
			"attempts", attempt,
			"duration", time.Since(startTime).String())

		return err
	}

	// Cleanup fallback client if it was created
	if fallbackClient != nil {
		fallbackClient.Close()
	}

	// Этот код не должен выполниться, но компилятор требует возврат значения
	return fmt.Errorf("unexpected execution path in processBlockHeader")
}

// processBlock обрабатывает блок и ищет релевантные транзакции
func (bsc *BinanceSmartChain) processBlock(ctx context.Context, client *ethclient.Client, header *types.Header) error {
	// Начинаем отсчет времени обработки блока
	startTime := time.Now()

	// Get the block
	block, err := client.BlockByHash(ctx, header.Hash())
	if err != nil {
		bsc.logger.ErrorContext(ctx, "Failed to get block",
			"error", err,
			"block_hash", header.Hash().Hex(),
			"duration", time.Since(startTime).String())
		return err
	}

	blockNumber := block.NumberU64()
	blockHash := block.Hash().Hex()

	var networkType string
	if shared.IsBlockchainDebugMode() {
		networkType = "Testnet"
	} else {
		networkType = "Mainnet"
	}

	bsc.logger.DebugContext(ctx, "Processing block",
		"block_number", blockNumber,
		"network", networkType,
		"usdt_contract", USDTContractAddress)

	logFields := LogFields{
		BlockNumber: blockNumber,
		BlockHash:   blockHash,
		Timestamp:   time.Now(),
	}

	// bsc.logger.InfoContext(ctx, "Processing new block",
	//	"block_number", blockNumber,
	//	"block_hash", blockHash,
	//	"tx_count", len(block.Transactions()),
	//	"timestamp", time.Now().Format(time.RFC3339))

	for i, tx := range block.Transactions() {
		txID := uuid.New().String() // Генерируем уникальный ID для отслеживания транзакции
		txHash := tx.Hash().Hex()

		// Заполняем информацию о транзакции
		txLogFields := logFields
		txLogFields.TxID = txID
		txLogFields.TxHash = txHash

		// Check if this is a transaction to the USDT contract
		if tx.To() != nil && tx.To().Hex() == USDTContractAddress {
			txLogFields.Contract = USDTContractAddress
			txLogFields.To = tx.To().Hex()

			// Get the input data
			data := tx.Data()

			// Check if this is a transfer call (first 4 bytes match the transfer signature)
			if len(data) >= 4 && bytes.Equal(data[:4], transferSig) {
				// Parse the transfer parameters
				if len(data) >= 4+32+32 { // 4 bytes for method ID, 32 bytes for each parameter
					// Extract recipient address (second parameter, padded to 32 bytes)
					recipientBytes := data[4:36]
					recipient := common.BytesToAddress(recipientBytes[12:]) // Remove padding
					recipientAddr := recipient.Hex()

					// Extract amount (third parameter)
					amountBytes := data[36:68]
					amount := new(big.Int).SetBytes(amountBytes)

					// Обновляем данные логирования
					txLogFields.To = recipientAddr
					txLogFields.Amount = amount.String()

					// Get the sender address
					sender, err := client.TransactionSender(ctx, tx, block.Hash(), uint(i))
					if err != nil {
						bsc.logger.ErrorContext(ctx, "Failed to get transaction sender",
							"error", err,
							"tx_id", txID,
							"tx_hash", txHash)
						continue
					}

					txLogFields.From = sender.Hex()

					// Check if the recipient is one of our wallets
					isOurWallet, err := bsc.wallets.IsOurWallet(ctx, recipientAddr)
					if err != nil {
						bsc.logger.ErrorContext(ctx, "Failed to check if wallet is tracked",
							"error", err,
							"tx_id", txID,
							"tx_hash", txHash,
							"recipient", recipientAddr)
						continue
					}

					if isOurWallet {
						bsc.logger.InfoContext(ctx, "USDT Transfer to our wallet detected",
							"tx_id", txID,
							"tx_hash", txHash,
							"from", sender.Hex(),
							"to", recipientAddr,
							"amount", amount.String(),
							"block_number", blockNumber,
							"status", TxStatusPending)

						// Выполняем AML проверку транзакции
						if bsc.amlService != nil {
							amlResult, amlErr := bsc.amlService.CheckTransaction(ctx, tx.Hash(), sender.Hex(), recipientAddr, amount)
							if amlErr != nil {
								bsc.logger.ErrorContext(ctx, "AML check failed",
									"error", amlErr,
									"tx_id", txID,
									"tx_hash", txHash)
								// Продолжаем обработку даже при ошибке AML проверки
							} else {
								bsc.logger.InfoContext(ctx, "AML check completed",
									"tx_id", txID,
									"tx_hash", txHash,
									"risk_level", amlResult.RiskLevel,
									"risk_score", amlResult.RiskScore,
									"approved", amlResult.Approved)

								// Если транзакция не одобрена по AML, отмечаем её в системе
								if !amlResult.Approved {
									bsc.logger.WarnContext(ctx, "Transaction flagged by AML check",
										"tx_id", txID,
										"tx_hash", txHash,
										"risk_level", amlResult.RiskLevel,
										"risk_source", amlResult.RiskSource,
										"requires_review", amlResult.RequiresReview,
										"notes", amlResult.Notes)

									// Обновляем статус транзакции
									err = bsc.transactions.MarkTransactionAMLFlagged(ctx, txHash)
									if err != nil {
										bsc.logger.ErrorContext(ctx, "Failed to mark transaction as AML flagged",
											"error", err,
											"tx_hash", txHash)
									}

									// Получаем и обновляем статус связанного ордера
									if bsc.orders != nil {
										orderID, err := bsc.wallets.GetOrderIdForWallet(ctx, recipientAddr)
										if err != nil {
											bsc.logger.ErrorContext(ctx, "Failed to get order for wallet",
												"error", err,
												"wallet", recipientAddr)
										} else {
											err = bsc.orders.MarkOrderForAMLReview(ctx, orderID, amlResult.Notes)
											if err != nil {
												bsc.logger.ErrorContext(ctx, "Failed to mark order for AML review",
													"error", err,
													"order_id", orderID)
											}
										}
									}
								}
							}
						}

						// Record the transaction
						if err = bsc.transactions.RecordTransaction(ctx, tx.Hash(), recipientAddr, amount, int64(blockNumber)); err != nil {
							bsc.logger.ErrorContext(ctx, "Failed to record transaction",
								"error", err,
								"tx_id", txID,
								"tx_hash", txHash)
						}

						// Check confirmations after RequiredConfirmations blocks
						// Используем семафор для ограничения количества одновременных проверок
						bsc.scheduleConfirmationCheck(ctx, client, tx.Hash(), blockNumber, txID)
					}
				}
			}
		}
	}

	// Логируем общее время обработки блока
	// bsc.logger.InfoContext(ctx, "Block processing completed",
	//	"block_number", blockNumber,
	//	"block_hash", blockHash,
	//	"duration", time.Since(startTime).String(),
	//	"tx_processed", len(block.Transactions()))

	return nil
}

// scheduleConfirmationCheck планирует проверку подтверждений с использованием семафора
func (bsc *BinanceSmartChain) scheduleConfirmationCheck(
	ctx context.Context,
	client *ethclient.Client,
	txHash common.Hash,
	blockNumber uint64,
	txID string,
) {
	// Создаем отдельную горутину для ожидания доступного слота в семафоре
	go func() {
		// Ожидаем доступный слот или завершение контекста
		select {
		case <-ctx.Done():
			return
		case bsc.confirmationSemaphore <- struct{}{}:
			// Слот получен, запускаем проверку подтверждений
			go func() {
				defer func() { <-bsc.confirmationSemaphore }() // Освобождаем слот после выполнения
				bsc.checkConfirmations(ctx, client, txHash, blockNumber, txID)
			}()
		}
	}()
}

// checkConfirmations ждет требуемого количества подтверждений и затем подтверждает транзакцию.
func (bsc *BinanceSmartChain) checkConfirmations(
	ctx context.Context,
	client *ethclient.Client,
	txHash common.Hash,
	blockNumber uint64,
	txID string, // Добавлен параметр txID для связывания логов
) {
	// Создаём контекст с метками для отслеживания
	startTime := time.Now()
	txHashHex := txHash.Hex()

	// Create a ticker to check every 30 seconds
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			bsc.logger.InfoContext(ctx, "Confirmation check cancelled",
				"tx_id", txID,
				"tx_hash", txHashHex,
				"reason", ctx.Err().Error(),
				"duration", time.Since(startTime).String())
			return
		case <-ticker.C:
			// Get current block number
			currentBlock, err := client.BlockNumber(ctx)
			if err != nil {
				bsc.logger.ErrorContext(ctx, "Failed to get current block number",
					"error", err,
					"tx_id", txID,
					"tx_hash", txHashHex)
				continue
			}

			// Рассчитываем количество подтверждений
			confirmations := currentBlock - blockNumber

			// Check if we have enough confirmations
			if confirmations >= bsc.config.Blockchain.RequiredConfirmations {
				// Confirm the transaction
				if err = bsc.transactions.ConfirmTransaction(ctx, txHashHex); err != nil {
					bsc.logger.ErrorContext(ctx, "Failed to confirm transaction",
						"error", err,
						"tx_id", txID,
						"tx_hash", txHashHex,
						"confirmations", confirmations,
						"status", TxStatusFailed,
						"duration", time.Since(startTime).String())
				} else {
					bsc.logger.InfoContext(ctx, "Transaction confirmed",
						"tx_id", txID,
						"tx_hash", txHashHex,
						"confirmations", confirmations,
						"status", TxStatusConfirmed,
						"duration", time.Since(startTime).String())
				}
				return
			}

			bsc.logger.InfoContext(ctx, "Waiting for confirmations",
				"tx_id", txID,
				"tx_hash", txHashHex,
				"current", confirmations,
				"required", bsc.config.Blockchain.RequiredConfirmations,
				"status", TxStatusPending,
				"elapsed_time", time.Since(startTime).String())
		}
	}
}

// getHTTPClient создает HTTP-клиент для взаимодействия с блокчейном
func getHTTPClient(ctx context.Context, logger *slog.Logger) (*ethclient.Client, error) {
	var client *ethclient.Client
	var err, lastErr error

	// Пробуем подключиться к разным эндпоинтам
	for _, endpoint := range bscHTTPEndpoints {
		logger.InfoContext(ctx, "Trying to connect to HTTP endpoint", "endpoint", endpoint)

		client, err = ethclient.DialContext(ctx, endpoint)
		if err == nil {
			logger.InfoContext(ctx, "Successfully connected to HTTP endpoint", "endpoint", endpoint)
			return client, nil
		}

		lastErr = err
		logger.WarnContext(ctx, "Failed to connect to HTTP endpoint",
			"endpoint", endpoint, "error", err)
	}

	return nil, fmt.Errorf("failed to connect to any HTTP endpoint: %w", lastErr)
}
