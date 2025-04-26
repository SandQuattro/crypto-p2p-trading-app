package aml

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"sync"
	"time"

	tx "github.com/Thiht/transactor/pgx"
	"github.com/ethereum/go-ethereum/common"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/aml/entities"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/aml/repository"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/aml/services"
)

// AMLService представляет основной сервис для AML проверок
type AMLService struct {
	logger      *slog.Logger
	repo        *repository.AMLRepository
	chainalysis *services.ChainalysisService
	elliptic    *services.EllipticService
	local       *services.LocalAMLService
	amlbot      *services.AMLBotService
	txService   TransactionService
	transactor  *tx.Transactor

	// Семафор для ограничения одновременных внешних проверок
	checkSemaphore chan struct{}
}

// TransactionService интерфейс для работы с транзакциями
type TransactionService interface {
	MarkTransactionAMLFlagged(ctx context.Context, txHash string) error
}

// NewAMLService создает новый сервис AML
func NewAMLService(
	logger *slog.Logger,
	repo *repository.AMLRepository,
	chainalysis *services.ChainalysisService,
	elliptic *services.EllipticService,
	local *services.LocalAMLService,
	amlbot *services.AMLBotService,
	txService TransactionService,
	transactor *tx.Transactor,
) *AMLService {
	return &AMLService{
		logger:         logger,
		repo:           repo,
		chainalysis:    chainalysis,
		elliptic:       elliptic,
		local:          local,
		amlbot:         amlbot,
		txService:      txService,
		transactor:     transactor,
		checkSemaphore: make(chan struct{}, 5), // Максимум 5 одновременных внешних проверок
	}
}

// CheckTransaction выполняет AML проверку транзакции
func (s *AMLService) CheckTransaction(ctx context.Context, txHash common.Hash, sourceAddress, destinationAddress string, amount *big.Int) (*entities.AMLCheckResult, error) {
	txHashStr := txHash.Hex()
	amountStr := amount.String()

	// Логируем начало проверки
	s.logger.InfoContext(ctx, "Starting AML check for transaction",
		"tx_hash", txHashStr,
		"source", sourceAddress,
		"destination", destinationAddress,
		"amount", amountStr)

	// Проверяем, есть ли уже результаты проверки для этой транзакции
	existingResult, err := s.repo.GetCheckResultByTxHash(ctx, txHashStr)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to check for existing AML results",
			"error", err,
			"tx_hash", txHashStr)
		// Продолжаем работу несмотря на ошибку
	} else if existingResult != nil {
		s.logger.InfoContext(ctx, "Found existing AML check result",
			"tx_hash", txHashStr,
			"risk_level", existingResult.RiskLevel,
			"approved", existingResult.Approved)
		return existingResult, nil
	}

	// Сохраняем транзакцию в очередь на проверку
	check := &entities.TransactionCheck{
		TxHash:        txHashStr,
		WalletAddress: destinationAddress,
		SourceAddress: sourceAddress,
		Amount:        amountStr,
		CreatedAt:     time.Now(),
		Processed:     false,
	}

	if err := s.repo.AddTransactionForChecking(ctx, check); err != nil {
		s.logger.ErrorContext(ctx, "Failed to add transaction to check queue",
			"error", err,
			"tx_hash", txHashStr)
		// Продолжаем работу несмотря на ошибку
	}

	// Запускаем все доступные проверки параллельно
	var wg sync.WaitGroup
	resultChan := make(chan *entities.AMLCheckResult, 3) // Для 3 потенциальных результатов
	errorChan := make(chan error, 3)

	// Всегда выполняем локальную проверку
	wg.Add(1)
	go func() {
		defer wg.Done()
		result, err := s.local.CheckTransaction(ctx, txHashStr, sourceAddress, destinationAddress, amountStr)
		if err != nil {
			errorChan <- fmt.Errorf("local AML check failed: %w", err)
			return
		}
		resultChan <- result
	}()

	// Проверка через Chainalysis, если сервис активирован
	if s.chainalysis.IsEnabled() {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Используем семафор для ограничения одновременных внешних запросов
			s.checkSemaphore <- struct{}{}
			defer func() { <-s.checkSemaphore }()

			result, err := s.chainalysis.CheckTransaction(ctx, txHashStr, sourceAddress, destinationAddress, amountStr)
			if err != nil {
				errorChan <- fmt.Errorf("chainalysis check failed: %w", err)
				return
			}
			resultChan <- result
		}()
	}

	// Проверка через Elliptic, если сервис активирован
	if s.elliptic.IsEnabled() {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Используем семафор для ограничения одновременных внешних запросов
			s.checkSemaphore <- struct{}{}
			defer func() { <-s.checkSemaphore }()

			result, err := s.elliptic.CheckTransaction(ctx, txHashStr, sourceAddress, destinationAddress, amountStr)
			if err != nil {
				errorChan <- fmt.Errorf("elliptic check failed: %w", err)
				return
			}
			resultChan <- result
		}()
	}

	// Проверка через AMLBot, если сервис активирован
	if s.amlbot != nil && s.amlbot.IsEnabled() {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Используем семафор для ограничения одновременных внешних запросов
			s.checkSemaphore <- struct{}{}
			defer func() { <-s.checkSemaphore }()

			result, err := s.amlbot.CheckTransaction(ctx, txHashStr, sourceAddress, destinationAddress, amountStr)
			if err != nil {
				errorChan <- fmt.Errorf("amlbot check failed: %w", err)
				return
			}
			resultChan <- result
		}()
	}

	// Ждем завершения всех проверок
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// Собираем и логируем ошибки
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
		s.logger.ErrorContext(ctx, "AML check error", "error", err, "tx_hash", txHashStr)
	}

	// Собираем результаты и выбираем самый строгий
	var finalResult *entities.AMLCheckResult
	var highestRiskScore float64
	var servicesUsed []string

	for result := range resultChan {
		if finalResult == nil || result.RiskScore > highestRiskScore {
			finalResult = result
			highestRiskScore = result.RiskScore
		}

		// Собираем информацию о использованных сервисах
		servicesUsed = append(servicesUsed, result.ExternalServicesUsed...)
	}

	// Если не получили ни одного результата, возвращаем ошибку
	if finalResult == nil {
		errMsg := "all AML checks failed"
		if len(errors) > 0 {
			errMsg = fmt.Sprintf("%s: %v", errMsg, errors[0])
		}
		return nil, fmt.Errorf(errMsg)
	}

	// Дополняем информацию о всех использованных сервисах
	finalResult.ExternalServicesUsed = servicesUsed

	// Сохраняем результат в базу и обновляем статус транзакции в одной транзакции
	err = s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		// Сохраняем результат проверки
		if err := s.repo.SaveCheckResult(txCtx, finalResult); err != nil {
			return fmt.Errorf("failed to save AML check result: %w", err)
		}

		// Отмечаем транзакцию как обработанную в таблице AML
		if err := s.repo.MarkCheckAsProcessed(txCtx, txHashStr); err != nil {
			return fmt.Errorf("failed to mark transaction as processed: %w", err)
		}

		// Если транзакция не прошла проверку, обновляем её статус в основной таблице transactions
		if !finalResult.Approved && s.txService != nil {
			if err := s.txService.MarkTransactionAMLFlagged(txCtx, txHashStr); err != nil {
				return fmt.Errorf("failed to update transaction AML status: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to save AML check results",
			"error", err,
			"tx_hash", txHashStr)
		// Продолжаем работу несмотря на ошибку сохранения,
		// так как сам результат проверки у нас уже есть
	}

	s.logger.InfoContext(ctx, "AML check completed",
		"tx_hash", txHashStr,
		"risk_level", finalResult.RiskLevel,
		"risk_score", finalResult.RiskScore,
		"approved", finalResult.Approved,
		"requires_review", finalResult.RequiresReview,
		"services_used", finalResult.ExternalServicesUsed)

	return finalResult, nil
}

// CheckAddress выполняет AML проверку адреса
func (s *AMLService) CheckAddress(ctx context.Context, address string) (*entities.AddressRiskInfo, error) {
	// Проверяем, есть ли информация в кэше
	cachedInfo, err := s.repo.GetAddressRiskInfo(ctx, address)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to get cached address risk info",
			"error", err,
			"address", address)
		// Продолжаем работу несмотря на ошибку
	} else if cachedInfo != nil && time.Since(cachedInfo.LastChecked) < 24*time.Hour {
		// Используем кэшированный результат, если он не старше 24 часов
		s.logger.InfoContext(ctx, "Using cached address risk info",
			"address", address,
			"risk_level", cachedInfo.RiskLevel,
			"last_checked", cachedInfo.LastChecked)
		return cachedInfo, nil
	}

	// Всегда выполняем локальную проверку
	localResult, err := s.local.CheckAddress(ctx, address)
	if err != nil {
		s.logger.ErrorContext(ctx, "Local address check failed",
			"error", err,
			"address", address)
		return nil, fmt.Errorf("local address check failed: %w", err)
	}

	// Если доступны внешние сервисы, пробуем использовать их для более точной проверки
	var externalResult *entities.AddressRiskInfo

	if s.chainalysis.IsEnabled() {
		s.checkSemaphore <- struct{}{}
		chainalysisResult, chainalysisErr := s.chainalysis.CheckAddress(ctx, address)
		<-s.checkSemaphore

		if chainalysisErr != nil {
			s.logger.ErrorContext(ctx, "Chainalysis address check failed",
				"error", chainalysisErr,
				"address", address)
		} else if chainalysisResult.RiskScore > localResult.RiskScore {
			externalResult = chainalysisResult
		}
	}

	if externalResult == nil && s.elliptic.IsEnabled() {
		s.checkSemaphore <- struct{}{}
		ellipticResult, ellipticErr := s.elliptic.CheckAddress(ctx, address)
		<-s.checkSemaphore

		if ellipticErr != nil {
			s.logger.ErrorContext(ctx, "Elliptic address check failed",
				"error", ellipticErr,
				"address", address)
		} else if ellipticResult.RiskScore > localResult.RiskScore {
			externalResult = ellipticResult
		}
	}

	// Проверка через AMLBot, если сервис активирован
	if externalResult == nil && s.amlbot != nil && s.amlbot.IsEnabled() {
		s.checkSemaphore <- struct{}{}
		amlbotResult, amlbotErr := s.amlbot.CheckAddress(ctx, address)
		<-s.checkSemaphore

		if amlbotErr != nil {
			s.logger.ErrorContext(ctx, "AMLBot address check failed",
				"error", amlbotErr,
				"address", address)
		} else if amlbotResult.RiskScore > localResult.RiskScore {
			externalResult = amlbotResult
		}
	}

	// Выбираем результат с наивысшим риском
	var finalResult *entities.AddressRiskInfo
	if externalResult != nil && externalResult.RiskScore > localResult.RiskScore {
		finalResult = externalResult
	} else {
		finalResult = localResult
	}

	// Сохраняем результат в кэш
	if err := s.repo.SaveAddressRiskInfo(ctx, finalResult); err != nil {
		s.logger.ErrorContext(ctx, "Failed to save address risk info",
			"error", err,
			"address", address)
	}

	s.logger.InfoContext(ctx, "Address risk check completed",
		"address", address,
		"risk_level", finalResult.RiskLevel,
		"risk_score", finalResult.RiskScore)

	return finalResult, nil
}

// ProcessPendingChecks обрабатывает очередь ожидающих AML-проверок транзакций
func (s *AMLService) ProcessPendingChecks(ctx context.Context) error {
	checks, err := s.repo.GetPendingChecks(ctx, 50) // Ограничиваем максимальное количество
	if err != nil {
		return fmt.Errorf("failed to get pending checks: %w", err)
	}

	if len(checks) == 0 {
		return nil // Нет транзакций для проверки
	}

	s.logger.InfoContext(ctx, "Processing pending AML checks", "count", len(checks))

	var wg sync.WaitGroup
	for _, check := range checks {
		wg.Add(1)
		go func(c entities.TransactionCheck) {
			defer wg.Done()

			checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			// Парсинг хеша транзакции
			txHash := common.HexToHash(c.TxHash)

			// Парсинг количества
			amount, ok := new(big.Int).SetString(c.Amount, 10)
			if !ok {
				s.logger.ErrorContext(ctx, "Failed to parse amount",
					"tx_hash", c.TxHash,
					"amount", c.Amount)
				amount = big.NewInt(0)
			}

			// Выполняем проверку
			_, err := s.CheckTransaction(checkCtx, txHash, c.SourceAddress, c.WalletAddress, amount)
			if err != nil {
				s.logger.ErrorContext(ctx, "Failed to process pending check",
					"error", err,
					"tx_hash", c.TxHash)
				// Несмотря на ошибку, отмечаем как обработанную, чтобы не застрять в цикле
				if markErr := s.repo.MarkCheckAsProcessed(ctx, c.TxHash); markErr != nil {
					s.logger.ErrorContext(ctx, "Failed to mark failed check as processed",
						"error", markErr,
						"tx_hash", c.TxHash)
				}
			}
		}(check)
	}

	wg.Wait()
	s.logger.InfoContext(ctx, "Completed processing pending AML checks", "count", len(checks))

	return nil
}

// StartBackgroundProcessing запускает фоновую обработку очереди AML-проверок
func (s *AMLService) StartBackgroundProcessing(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	s.logger.Info("Starting background AML checks processing")

	// Выполняем начальную обработку
	if err := s.ProcessPendingChecks(ctx); err != nil {
		s.logger.Error("Failed to process initial pending AML checks", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Stopping background AML checks processing")
			return
		case <-ticker.C:
			if err := s.ProcessPendingChecks(ctx); err != nil {
				s.logger.Error("Failed to process pending AML checks", "error", err)
			}
		}
	}
}
