package workers

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/sand/crypto-p2p-trading-app/backend/config"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
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

type TransactionService interface {
	GetTransactionsByWallet(ctx context.Context, walletAddress string) ([]entities.Transaction, error)
	RecordTransaction(ctx context.Context, txHash common.Hash, walletAddress string, amount *big.Int, blockNumber int64) error
	ConfirmTransaction(ctx context.Context, txHash string) error
	ProcessPendingTransactions(ctx context.Context) error
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
}

const (
	USDTContractAddress    = "0x55d398326f99059fF775485246999027B3197955" // USDT BEP-20 контракт
	subscriptionRetryDelay = 10 * time.Second                             // Delay before retrying subscription
)

// Define the ERC-20 transfer method signature.
var (
	transferSig = []byte{0xa9, 0x05, 0x9c, 0xbb} // keccak256("transfer(address,uint256)")[0:4]
)

type BinanceSmartChain struct {
	logger *slog.Logger
	config *config.Config

	transactions TransactionService
	wallets      WalletService
}

func NewBinanceSmartChain(
	logger *slog.Logger,
	config *config.Config,
	transactions TransactionService,
	wallets WalletService,
) *BinanceSmartChain {
	return &BinanceSmartChain{
		logger:       logger,
		config:       config,
		transactions: transactions,
		wallets:      wallets,
	}
}

// SubscribeToTransactions monitors incoming transactions via Web3.
// The service will poll for new blocks and process incoming transactions.
func (bsc *BinanceSmartChain) SubscribeToTransactions(ctx context.Context, rpcURL string) {
	for {
		bsc.logger.InfoContext(ctx, "Starting blockchain monitoring...", "rpc_url", rpcURL)

		if err := bsc.pollAndProcess(ctx, rpcURL); err != nil {
			bsc.logger.InfoContext(ctx, "Blockchain monitoring error, retrying...",
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

func (bsc *BinanceSmartChain) pollAndProcess(ctx context.Context, rpcURL string) error {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return fmt.Errorf("failed to connect to Ethereum client: %w", err)
	}
	defer client.Close()

	// Process pending transactions every minute
	processTicker := time.NewTicker(1 * time.Minute)
	defer processTicker.Stop()

	// Poll for new blocks every 5 seconds
	pollTicker := time.NewTicker(5 * time.Second)
	defer pollTicker.Stop()

	var lastProcessedBlock uint64

	// Get current block number to start from
	currentBlock, err := client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current block number: %w", err)
	}

	lastProcessedBlock = currentBlock
	bsc.logger.InfoContext(ctx, "Starting blockchain monitoring from block", "block", currentBlock)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("pullAndProcess done with %w", ctx.Err())
		case <-processTicker.C:
			if err = bsc.transactions.ProcessPendingTransactions(ctx); err != nil {
				bsc.logger.ErrorContext(ctx, "Failed to process pending transactions", "error", err)
			}
		case <-pollTicker.C:
			// Get latest block number
			latestBlock, e := client.BlockNumber(ctx)
			if e != nil {
				bsc.logger.ErrorContext(ctx, "Failed to get latest block number", "error", e)
				continue
			}

			// Process new blocks
			if latestBlock > lastProcessedBlock {
				// bsc.logger.InfoContext(ctx,"New blocks detected", "from", lastProcessedBlock+1, "to", latestBlock)

				// Process each new block
				for blockNum := lastProcessedBlock + 1; blockNum <= latestBlock; blockNum++ {
					block, err := client.BlockByNumber(ctx, big.NewInt(int64(blockNum)))
					if err != nil {
						bsc.logger.ErrorContext(ctx, "Failed to get block", "block", blockNum, "error", err)
						continue
					}

					bsc.processBlock(ctx, client, block.Header())
				}

				lastProcessedBlock = latestBlock
			}
		}
	}
}

// processBlock обрабатывает блок и ищет релевантные транзакции
func (bsc *BinanceSmartChain) processBlock(ctx context.Context, client *ethclient.Client, header *types.Header) {
	// Начинаем отсчет времени обработки блока
	startTime := time.Now()

	// Get the block
	block, err := client.BlockByHash(ctx, header.Hash())
	if err != nil {
		bsc.logger.ErrorContext(ctx, "Failed to get block",
			"error", err,
			"block_hash", header.Hash().Hex(),
			"duration", time.Since(startTime).String())
		return
	}

	blockNumber := block.NumberU64()
	blockHash := block.Hash().Hex()

	logFields := LogFields{
		BlockNumber: blockNumber,
		BlockHash:   blockHash,
		Timestamp:   time.Now(),
	}

	bsc.logger.InfoContext(ctx, "Processing new block",
		"block_number", blockNumber,
		"block_hash", blockHash,
		"tx_count", len(block.Transactions()),
		"timestamp", time.Now().Format(time.RFC3339))

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

						// Record the transaction
						if err = bsc.transactions.RecordTransaction(ctx, tx.Hash(), recipientAddr, amount, int64(blockNumber)); err != nil {
							bsc.logger.ErrorContext(ctx, "Failed to record transaction",
								"error", err,
								"tx_id", txID,
								"tx_hash", txHash)
						}

						// Check confirmations after RequiredConfirmations blocks
						go bsc.checkConfirmations(ctx, client, tx.Hash(), blockNumber, txID)
					}
				}
			}
		}
	}

	// Логируем общее время обработки блока
	bsc.logger.InfoContext(ctx, "Block processing completed",
		"block_number", blockNumber,
		"block_hash", blockHash,
		"duration", time.Since(startTime).String(),
		"tx_processed", len(block.Transactions()))
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
