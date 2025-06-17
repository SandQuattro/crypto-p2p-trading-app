package workers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	"github.com/sand/crypto-p2p-trading-app/backend/config"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/core/ports"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/shared"
	"go.openly.dev/pointy"
)

// GetSolanaWebSocketEndpoints Solana WebSocket endpoints
func GetSolanaWebSocketEndpoints() []string {
	if shared.IsBlockchainDebugMode() {
		// Devnet WebSocket endpoints for debug/test mode
		return []string{
			rpc.DevNet_WS,
			// можно добавить резервные эндпоинты Devnet при необходимости
		}
	}
	// Mainnet WebSocket endpoints for production
	return []string{
		rpc.MainNetBeta_WS,
		// можно добавить резервные эндпоинты Mainnet при необходимости
	}
}

// GetSolanaHTTPEndpoints Solana HTTP endpoints
func GetSolanaHTTPEndpoints() []string {
	if shared.IsBlockchainDebugMode() {
		// Devnet HTTP endpoints for debug/test mode
		return []string{
			rpc.DevNet_RPC,
			// можно добавить резервные эндпоинты Devnet при необходимости
		}
	}
	// Mainnet HTTP endpoints for production
	return []string{
		rpc.MainNetBeta_RPC,
		// можно добавить резервные эндпоинты Mainnet при необходимости
	}
}

// GetSPLTokenAddress returns the appropriate SPL token address based on mode
// Для примера используется USDT на Solana. В будущем можно сделать это настраиваемым.
func GetSPLTokenAddress() string {
	if shared.IsBlockchainDebugMode() {
		// Адрес USDT (или другого токена) в Devnet.
		// Необходимо найти и указать актуальный адрес для Devnet, если он отличается.
		// Пока оставим такой же, как для Mainnet, для примера.
		return "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB" // USDT on Solana
	}
	return "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB" // USDT on Solana Mainnet
}

// SPLTokenAddress returns the appropriate SPL Token address based on mode
var SPLTokenAddress = GetSPLTokenAddress()

// SolanaBlockchain handles Solana blockchain operations.
type SolanaBlockchain struct {
	logger *slog.Logger
	config *config.Config

	transactions ports.TransactionService
	wallets      ports.WalletService
	amlService   ports.AMLService
	orders       ports.OrderService

	// Семафор для ограничения одновременных проверок подтверждений (если применимо для Solana)
	confirmationSemaphore chan struct{}

	mu                sync.Mutex
	lastProcessedSlot uint64 // В Solana "блоки" называются "слотами"
}

// NewSolanaBlockchain creates a new SolanaBlockchain instance.
func NewSolanaBlockchain(
	logger *slog.Logger,
	config *config.Config,
	transactions ports.TransactionService,
	wallets ports.WalletService,
	amlService ports.AMLService,
	orders ports.OrderService,
) *SolanaBlockchain {
	SPLTokenAddress = GetSPLTokenAddress() // Обновляем адрес токена

	networkName := "Mainnet"
	if shared.IsBlockchainDebugMode() {
		networkName = "Devnet"
	}

	logger.Info("Initializing Solana blockchain monitoring",
		"network", networkName,
		"spl_token_address", SPLTokenAddress)

	return &SolanaBlockchain{
		logger:                logger,
		config:                config,
		transactions:          transactions,
		wallets:               wallets,
		amlService:            amlService,
		orders:                orders,
		confirmationSemaphore: make(chan struct{}, ports.MaxConcurrentChecks),
	}
}

// SubscribeToTransactions monitors incoming transactions.
// В Solana это будет подписка на новые слоты (блоки).
func (s *SolanaBlockchain) SubscribeToTransactions(ctx context.Context) {
	for {
		s.logger.InfoContext(ctx, "Starting Solana blockchain monitoring via WebSocket...")

		if err := s.subscribeViaWebsocket(ctx); err != nil {
			s.logger.ErrorContext(ctx, "Solana WebSocket subscription failed, retrying...",
				"delay", ports.BlockchainSubscriptionRetryDelay, "error", err)

			select {
			case <-ctx.Done():
				return
			case <-time.After(ports.BlockchainSubscriptionRetryDelay):
				continue
			}
		}
		return // Успешная подписка
	}
}

// subscribeViaWebsocket subscribes to new slots via WebSocket
func (s *SolanaBlockchain) subscribeViaWebsocket(ctx context.Context) error {
	var wsClient *ws.Client
	var wsEndpoint string
	var err error

	s.logger.InfoContext(ctx, "Attempting to connect to Solana via WebSocket")

	for _, endpoint := range GetSolanaWebSocketEndpoints() {
		s.logger.InfoContext(ctx, "Trying Solana WebSocket endpoint", "endpoint", endpoint)
		wsClient, err = ws.Connect(ctx, endpoint)
		if err != nil {
			s.logger.WarnContext(ctx, "Failed to connect to Solana WebSocket endpoint",
				"endpoint", endpoint, "error", err)
			continue
		}
		wsEndpoint = endpoint
		s.logger.InfoContext(ctx, "Successfully connected to Solana WebSocket endpoint", "endpoint", endpoint)
		break
	}

	if wsClient == nil {
		return fmt.Errorf("failed to connect to any Solana WebSocket endpoint")
	}
	defer wsClient.Close()

	// Получаем текущий номер слота для начала мониторинга (аналог currentBlock в BSC)
	// Для Solana это может быть не так критично, как для Ethereum-подобных сетей,
	// так как подписка на слоты обычно доставляет актуальные.
	// Но для полноты картины и возможной обработки пропущенных слотов можно добавить.
	rpcClient := rpc.New(GetSolanaHTTPEndpoints()[0]) // Используем первый HTTP-эндпоинт для запросов
	currentSlot, err := rpcClient.GetSlot(ctx, rpc.CommitmentFinalized)
	if err != nil {
		return fmt.Errorf("failed to get current slot number: %w", err)
	}

	s.mu.Lock()
	s.lastProcessedSlot = currentSlot
	s.mu.Unlock()

	s.logger.InfoContext(ctx, "Starting Solana WebSocket monitoring from slot",
		"slot", currentSlot, "endpoint", wsEndpoint)

	// Подписываемся на новые слоты
	sub, err := wsClient.SlotSubscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to new slots: %w", err)
	}
	defer sub.Unsubscribe()

	// HTTP-клиент для получения деталей слота/транзакций
	httpClient, err := getSolanaHTTPClient(ctx, s.logger)
	if err != nil {
		return fmt.Errorf("failed to create Solana HTTP client: %w", err)
	}
	defer httpClient.Close()

	for {
		// Сначала проверяем неблокирующие условия выхода или ошибки подписки
		select {
		case <-ctx.Done():
			return fmt.Errorf("Solana WebSocket subscription context cancelled: %w", ctx.Err())
		case err := <-sub.Err():
			return fmt.Errorf("Solana WebSocket subscription error (from sub.Err()): %w", err)
		default:
			// Если каналы не готовы, продолжаем к блокирующему вызову
		}

		// Теперь делаем блокирующий вызов sub.Recv(ctx)
		// Он сам должен уважать контекст и вернуть ошибку, если ctx.Done()
		slotInfo, err := sub.Recv(ctx)
		if err != nil {
			// Если Recv вернул ошибку, это может быть из-за отмены контекста или другой проблемы.
			s.logger.ErrorContext(ctx, "WebSocket Recv failed", "error", err, "endpoint", wsEndpoint)
			// Возвращаем ошибку, чтобы внешний цикл мог попытаться переподключиться.
			return fmt.Errorf("WebSocket Recv error from endpoint %s: %w", wsEndpoint, err)
		}

		if slotInfo == nil { // На всякий случай, если API может вернуть (nil, nil)
			s.logger.InfoContext(ctx, "Received nil slotInfo without error, skipping", "endpoint", wsEndpoint)
			continue
		}

		slotNumber := slotInfo.Slot

		s.mu.Lock()
		lastProcessed := s.lastProcessedSlot
		s.mu.Unlock()

		// Проверяем, не пропустили ли мы слоты (аналогично BSC)
		if slotNumber > lastProcessed+1 {
			s.logger.WarnContext(ctx, "Missed Solana slots detected, fetching missing slots",
				"from", lastProcessed+1, "to", slotNumber-1)
			for missedSlot := lastProcessed + 1; missedSlot < slotNumber; missedSlot++ {
				if err := s.processSlot(ctx, httpClient, missedSlot); err != nil {
					s.logger.ErrorContext(ctx, "Failed to process missed Solana slot", "slot", missedSlot, "error", err)
				}
			}
		}

		// Обрабатываем текущий слот
		if err := s.processSlot(ctx, httpClient, slotNumber); err != nil {
			s.logger.ErrorContext(ctx, "Failed to process Solana slot",
				"slot", slotNumber, "error", err)
		}

		// Обновляем последний обработанный слот
		s.mu.Lock()
		if slotNumber > s.lastProcessedSlot {
			s.lastProcessedSlot = slotNumber
		}
		s.mu.Unlock()

		// TODO: Периодически обрабатываем ожидающие транзакции (если такая логика нужна для Solana)
		// if err := s.transactions.ProcessPendingTransactions(ctx); err != nil {
		// 	s.logger.ErrorContext(ctx, "Failed to process pending transactions", "error", err)
		// }
	}
}

// getSolanaHTTPClient создает HTTP-клиент для взаимодействия с Solana.
func getSolanaHTTPClient(ctx context.Context, logger *slog.Logger) (*rpc.Client, error) {
	var client *rpc.Client
	var lastErr error

	for _, endpoint := range GetSolanaHTTPEndpoints() {
		logger.InfoContext(ctx, "Trying to connect to Solana HTTP endpoint", "endpoint", endpoint)
		// В библиотеке gagliardetto/solana-go RPC клиент создается напрямую
		client = rpc.New(endpoint)
		// Проверим соединение, запросив версию (или другой простой метод)
		_, err := client.GetVersion(ctx)
		if err == nil {
			logger.InfoContext(ctx, "Successfully connected to Solana HTTP endpoint", "endpoint", endpoint)
			return client, nil
		}
		lastErr = err
		logger.WarnContext(ctx, "Failed to connect to Solana HTTP endpoint", "endpoint", endpoint, "error", err)
	}
	return nil, fmt.Errorf("failed to connect to any Solana HTTP endpoint: %w", lastErr)
}

// processSlot обрабатывает слот (аналог блока в BSC)
func (s *SolanaBlockchain) processSlot(ctx context.Context, rpcClient *rpc.Client, slotNumber uint64) error {
	startTime := time.Now()
	s.logger.DebugContext(ctx, "Processing Solana slot", "slot_number", slotNumber)

	block, err := rpcClient.GetBlockWithOpts(ctx, slotNumber, &rpc.GetBlockOpts{
		Encoding:                       solana.EncodingBase64, // Используем Base64, будем декодировать вручную
		MaxSupportedTransactionVersion: pointy.Uint8(0),
		TransactionDetails:             rpc.TransactionDetailsFull,
		Commitment:                     rpc.CommitmentConfirmed,
		Rewards:                        pointy.Bool(false),
	})

	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to get Solana block/slot",
			"slot_number", slotNumber, "error", err, "duration", time.Since(startTime).String())
		return err
	}

	if block == nil {
		s.logger.InfoContext(ctx, "Solana block/slot is nil",
			"slot_number", slotNumber, "duration", time.Since(startTime).String())
		return nil
	}

	if block.Transactions == nil || len(block.Transactions) == 0 {
		s.logger.DebugContext(ctx, "Solana block/slot has no transactions",
			"slot_number", slotNumber, "block_hash", block.Blockhash.String(), "duration", time.Since(startTime).String())
		return nil
	}

	s.logger.InfoContext(ctx, "Processing Solana slot",
		"slot_number", slotNumber, "block_hash", block.Blockhash.String(),
		"tx_count", len(block.Transactions), "duration", time.Since(startTime).String())

	for _, txWithMeta := range block.Transactions {
		if txWithMeta.Transaction == nil {
			s.logger.WarnContext(ctx, "Transaction data (rpc.TransactionEncoded) is nil in slot", "slot_number", slotNumber)
			continue
		}

		// Получаем бинарные данные транзакции, так как запросили EncodingBase64
		binaryTxData, err := txWithMeta.Transaction.GetBinary()
		if err != nil {
			s.logger.WarnContext(ctx, "Failed to get binary transaction data in slot",
				"slot_number", slotNumber, "error", err)
			continue
		}

		// Декодируем бинарные данные в *solana.Transaction
		decodedTx, err := solana.TransactionFromDecoder(bin.NewBinDecoder(binaryTxData))
		if err != nil {
			s.logger.WarnContext(ctx, "Failed to decode binary transaction from slot",
				"slot_number", slotNumber, "error", err)
			continue
		}

		if decodedTx == nil || len(decodedTx.Signatures) == 0 {
			s.logger.WarnContext(ctx, "Decoded transaction is nil or has no signatures", "slot_number", slotNumber)
			continue
		}

		txSignature := decodedTx.Signatures[0].String()

		if txWithMeta.Meta == nil {
			s.logger.WarnContext(ctx, "Transaction metadata is nil", "tx_signature", txSignature, "slot_number", slotNumber)
			// Можно решить, продолжать ли без метаданных или нет. Пока пропустим.
			continue
		}

		if err := s.processTransaction(ctx, rpcClient, txSignature, slotNumber, decodedTx, txWithMeta.Meta); err != nil {
			s.logger.ErrorContext(ctx, "Failed to process Solana transaction",
				"tx_signature", txSignature, "slot_number", slotNumber, "error", err)
		}
	}

	return nil
}

// processTransaction обрабатывает транзакцию в сети Solana
func (s *SolanaBlockchain) processTransaction(ctx context.Context, rpcClient *rpc.Client, txSignature string, slotNumber uint64, decodedTx *solana.Transaction, txMeta *rpc.TransactionMeta) error {
	s.logger.InfoContext(ctx, "processTransaction for Solana (stub)", "tx_signature", txSignature, "slot_number", slotNumber)

	// decodedTx содержит инструкции и подписи
	// txMeta содержит логи, изменения балансов (PreBalances, PostBalances, PreTokenBalances, PostTokenBalances), статус и т.д.

	if decodedTx == nil || txMeta == nil {
		s.logger.WarnContext(ctx, "processTransaction called with nil decodedTx or txMeta", "tx_signature", txSignature)
		return fmt.Errorf("decodedTx or txMeta is nil for %s", txSignature)
	}

	// Проверяем статус транзакции по метаданным
	if txMeta.Err != nil {
		s.logger.InfoContext(ctx, "Transaction has failed status",
			"tx_signature", txSignature,
			"error", txMeta.Err)
		// TODO: Возможно, нужно записать эту транзакцию как неуспешную
		return nil // Не обрабатываем дальше, если транзакция не удалась
	}

	// Далее логика анализа транзакции, как в bsc_blockchain.go:
	// 1. Итерировать по инструкциям в decodedTx.Message.Instructions
	// 2. Найти инструкцию перевода SPL-токена (например, с помощью spltoken.ParseTransferCheckedInstruction или аналогичного).
	//    Для этого нужно знать ProgramID SPL Token Program (solana.TokenProgramID).
	// 3. Извлечь отправителя, получателя, сумму, адрес минта токена.
	// 4. Проверить, что адрес минта токена совпадает с отслеживаемым (SPLTokenAddress).
	// 5. Проверить, является ли получатель нашим кошельком (s.wallets.IsOurWallet).
	// 6. Если да, то:
	//    a. AML-проверка (s.amlService.CheckTransaction). Придется адаптировать параметры.
	//    b. Запись транзакции в БД (s.transactions.RecordTransaction). Адаптировать параметры (txHash для Solana это сигнатура).
	//    c. Запланировать проверку подтверждений (s.scheduleConfirmationCheck). Для Solana это может быть подписка на статус сигнатуры до 'finalized'.

	s.logger.InfoContext(ctx, "TODO: Implement SPL token transfer detection and processing", "tx_signature", txSignature)

	return nil
}

// checkConfirmations проверяет подтверждения транзакции в Solana
func (s *SolanaBlockchain) checkConfirmations(ctx context.Context, rpcClient *rpc.Client, txSignature string, initialSlotNumber uint64) {
	// TODO: Реализовать логику проверки статуса транзакции (processed, confirmed, finalized)
	// и подтверждение в нашей системе.
	s.logger.InfoContext(ctx, "checkConfirmations for Solana is not yet implemented", "tx_signature", txSignature)
}
