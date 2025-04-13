package services

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/aml/entities"
)

// LocalAMLService представляет сервис для локальных AML проверок без обращения к внешним API
type LocalAMLService struct {
	logger *slog.Logger

	// В реальной системе здесь могут быть локальные списки санкций и черные списки
	knownRiskyAddresses map[string]float64

	// Пороговые значения для срабатывания проверок
	transactionThreshold *big.Float
}

// NewLocalAMLService создает новый сервис для локальных AML проверок
func NewLocalAMLService(logger *slog.Logger, thresholdAmount string) *LocalAMLService {
	threshold, _ := new(big.Float).SetString(thresholdAmount)
	if threshold == nil {
		threshold = new(big.Float).SetFloat64(5000.0) // Значение по умолчанию, если не удалось распарсить
	}

	// Инициализируем тестовый список рискованных адресов
	riskyAddresses := make(map[string]float64)
	// Можно добавить известные адреса для тестирования
	riskyAddresses["0x123456789abcdef123456789abcdef123456789a"] = 0.9 // Высокий риск
	riskyAddresses["0xabcdef123456789abcdef123456789abcdef1234"] = 0.7 // Средний риск

	logger.Info("Initialized local AML service",
		"threshold", threshold.String(),
		"known_risky_addresses", len(riskyAddresses))

	return &LocalAMLService{
		logger:               logger,
		knownRiskyAddresses:  riskyAddresses,
		transactionThreshold: threshold,
	}
}

// CheckAddress проверяет адрес на риски локально
func (s *LocalAMLService) CheckAddress(ctx context.Context, address string) (*entities.AddressRiskInfo, error) {
	lowercaseAddress := strings.ToLower(address)

	// Проверяем, известен ли адрес как рискованный
	riskScore, known := s.knownRiskyAddresses[lowercaseAddress]

	var riskLevel entities.RiskLevel
	if known {
		if riskScore >= 0.7 {
			riskLevel = entities.RiskLevelHigh
		} else if riskScore >= 0.4 {
			riskLevel = entities.RiskLevelMedium
		} else {
			riskLevel = entities.RiskLevelLow
		}

		s.logger.InfoContext(ctx, "Found address in local risk database",
			"address", address,
			"risk_score", riskScore,
			"risk_level", riskLevel)
	} else {
		// Выполняем базовую эвристическую проверку адреса
		riskScore = s.analyzeAddressPattern(lowercaseAddress)

		if riskScore >= 0.7 {
			riskLevel = entities.RiskLevelHigh
		} else if riskScore >= 0.4 {
			riskLevel = entities.RiskLevelMedium
		} else {
			riskLevel = entities.RiskLevelLow
		}

		s.logger.InfoContext(ctx, "Address risk analysis completed",
			"address", address,
			"risk_score", riskScore,
			"risk_level", riskLevel)
	}

	return &entities.AddressRiskInfo{
		Address:     address,
		RiskLevel:   riskLevel,
		RiskScore:   riskScore,
		LastChecked: time.Now(),
		Category:    "local_check",
		Source:      "local_aml",
		Tags:        []string{"locally_verified"},
	}, nil
}

// analyzeAddressPattern выполняет эвристический анализ адреса
func (s *LocalAMLService) analyzeAddressPattern(address string) float64 {
	// Это очень примитивная эвристика для демонстрации
	// В реальной системе здесь могут быть более сложные алгоритмы

	// Пример: проверка на повторяющиеся паттерны в адресе
	if strings.Contains(address, "000000") {
		return 0.4 // Средний риск для адресов с повторяющимися нулями
	}

	// Пример: проверка на ванити-адреса
	if strings.Contains(address, "dead") || strings.Contains(address, "beef") {
		return 0.3 // Небольшой риск для известных ванити-паттернов
	}

	return 0.1 // Низкий риск по умолчанию
}

// CheckTransaction проверяет транзакцию на риски
func (s *LocalAMLService) CheckTransaction(ctx context.Context, txHash, sourceAddress, destinationAddress, amount string) (*entities.AMLCheckResult, error) {
	// Проверяем исходный адрес
	sourceRiskInfo, err := s.CheckAddress(ctx, sourceAddress)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to check source address",
			"error", err,
			"address", sourceAddress,
			"tx_hash", txHash)
		return nil, fmt.Errorf("failed to check source address: %w", err)
	}

	// Проверяем сумму транзакции
	amountRisk := s.checkTransactionAmount(amount)

	// Комбинируем результаты проверок
	finalRiskScore := sourceRiskInfo.RiskScore
	if amountRisk > finalRiskScore {
		finalRiskScore = amountRisk
	}

	var riskLevel entities.RiskLevel
	if finalRiskScore >= 0.7 {
		riskLevel = entities.RiskLevelHigh
	} else if finalRiskScore >= 0.4 {
		riskLevel = entities.RiskLevelMedium
	} else {
		riskLevel = entities.RiskLevelLow
	}

	notes := fmt.Sprintf("Source address risk: %.2f, Transaction amount risk: %.2f",
		sourceRiskInfo.RiskScore, amountRisk)

	result := &entities.AMLCheckResult{
		TransactionHash:      txHash,
		WalletAddress:        destinationAddress,
		SourceAddress:        sourceAddress,
		RiskLevel:            riskLevel,
		RiskSource:           entities.RiskSourceBehavioral,
		RiskScore:            finalRiskScore,
		Approved:             finalRiskScore < 0.7, // Пороговое значение для автоматического одобрения
		CheckedAt:            time.Now(),
		Notes:                notes,
		RequiresReview:       finalRiskScore >= 0.5, // Пороговое значение для ручного рассмотрения
		ExternalServicesUsed: []string{"local_aml"},
	}

	s.logger.InfoContext(ctx, "Transaction AML check completed locally",
		"tx_hash", txHash,
		"risk_level", result.RiskLevel,
		"risk_score", result.RiskScore,
		"approved", result.Approved,
		"requires_review", result.RequiresReview)

	return result, nil
}

// checkTransactionAmount проверяет риск на основе суммы транзакции
func (s *LocalAMLService) checkTransactionAmount(amount string) float64 {
	amountFloat, _ := new(big.Float).SetString(amount)
	if amountFloat == nil {
		return 0.5 // Средний риск по умолчанию при ошибке парсинга
	}

	// Проверяем, превышает ли сумма пороговое значение
	if amountFloat.Cmp(s.transactionThreshold) >= 0 {
		// Вычисляем риск в зависимости от того, насколько превышен порог
		ratio := new(big.Float).Quo(amountFloat, s.transactionThreshold)

		// Конвертируем соотношение в float64 для расчета риска
		ratioFloat, _ := ratio.Float64()

		// Ограничиваем максимальное значение риска
		if ratioFloat > 10 {
			return 0.9 // Максимальный риск для транзакций в 10+ раз выше порога
		}

		// Линейная функция риска от 0.5 до 0.9 в зависимости от соотношения
		return 0.5 + 0.04*ratioFloat
	}

	return 0.2 // Низкий риск для транзакций ниже порога
}
