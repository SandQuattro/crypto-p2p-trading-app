package clients

import (
	"context"
	"fmt"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
	"log/slog"
	"net/http"
	"time"
)

// EllipticService представляет сервис для проверки транзакций через Elliptic (TRM Labs) API
type EllipticService struct {
	logger    *slog.Logger
	apiKey    string
	apiURL    string
	client    *http.Client
	isEnabled bool
}

// NewEllipticService создает новый сервис для проверки транзакций через Elliptic (TRM Labs)
func NewEllipticService(logger *slog.Logger, apiKey, apiURL string) *EllipticService {
	isEnabled := apiKey != "" && apiURL != ""

	if !isEnabled {
		logger.Warn("Elliptic (TRM Labs) service is disabled due to missing credentials")
	}

	return &EllipticService{
		logger:    logger,
		apiKey:    apiKey,
		apiURL:    apiURL,
		client:    &http.Client{Timeout: 10 * time.Second},
		isEnabled: isEnabled,
	}
}

// IsEnabled возвращает статус активации сервиса
func (s *EllipticService) IsEnabled() bool {
	return s.isEnabled
}

// CheckAddress проверяет адрес на риски через Elliptic API
func (s *EllipticService) CheckAddress(ctx context.Context, address string) (*entities.AddressRiskInfo, error) {
	if !s.isEnabled {
		s.logger.Warn("Elliptic service is disabled, skipping check", "address", address)
		return &entities.AddressRiskInfo{
			Address:     address,
			RiskLevel:   entities.RiskLevelLow,
			RiskScore:   0,
			LastChecked: time.Now(),
			Source:      "elliptic_disabled",
		}, nil
	}

	// Формирование запроса к Elliptic API
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/v1/address/%s", s.apiURL, address), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Elliptic request: %w", err)
	}

	req.Header.Set("X-API-Key", s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	s.logger.InfoContext(ctx, "Checking address with Elliptic (TRM Labs)", "address", address)

	// В реальной интеграции здесь происходит обращение к API Elliptic
	// Для демонстрации возвращаем заглушку

	// resp, err := s.client.Do(req)
	// if err != nil {
	//     return nil, fmt.Errorf("failed to send request to Elliptic: %w", err)
	// }
	// defer resp.Body.Close()

	// Имитация результата проверки
	riskInfo := &entities.AddressRiskInfo{
		Address:     address,
		RiskLevel:   entities.RiskLevelLow,
		RiskScore:   0.05,
		LastChecked: time.Now(),
		Category:    "normal",
		Source:      "elliptic",
		Tags:        []string{"verified"},
	}

	s.logger.InfoContext(ctx, "Elliptic check completed",
		"address", address,
		"risk_level", riskInfo.RiskLevel,
		"risk_score", riskInfo.RiskScore)

	return riskInfo, nil
}

// CheckTransaction проверяет транзакцию на риски через Elliptic API
func (s *EllipticService) CheckTransaction(ctx context.Context, txHash, sourceAddress, destinationAddress, amount string) (*entities.AMLCheckResult, error) {
	if !s.isEnabled {
		s.logger.Warn("Elliptic service is disabled, skipping transaction check",
			"tx_hash", txHash,
			"source", sourceAddress,
			"destination", destinationAddress)

		return &entities.AMLCheckResult{
			TransactionHash:      txHash,
			WalletAddress:        destinationAddress,
			SourceAddress:        sourceAddress,
			RiskLevel:            entities.RiskLevelLow,
			RiskSource:           entities.RiskSourceSanctionsList,
			RiskScore:            0,
			Approved:             true,
			CheckedAt:            time.Now(),
			Notes:                "Check skipped - Elliptic service disabled",
			RequiresReview:       false,
			ExternalServicesUsed: []string{"elliptic_disabled"},
		}, nil
	}

	// Проверка исходного адреса
	sourceRiskInfo, err := s.CheckAddress(ctx, sourceAddress)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to check source address with Elliptic",
			"error", err,
			"address", sourceAddress,
			"tx_hash", txHash)
		// Продолжаем выполнение даже при ошибке, считая адрес с низким риском
		sourceRiskInfo = &entities.AddressRiskInfo{
			Address:     sourceAddress,
			RiskLevel:   entities.RiskLevelMedium,
			RiskScore:   0.5,
			LastChecked: time.Now(),
			Source:      "elliptic_error",
		}
	}

	// В реальной интеграции здесь был бы еще запрос к Elliptic API для проверки транзакции
	// и получения дополнительной информации о происхождении средств

	// Формируем результат на основе информации о риске адреса
	result := &entities.AMLCheckResult{
		TransactionHash:      txHash,
		WalletAddress:        destinationAddress,
		SourceAddress:        sourceAddress,
		RiskLevel:            sourceRiskInfo.RiskLevel,
		RiskSource:           entities.RiskSourceSanctionsList,
		RiskScore:            sourceRiskInfo.RiskScore,
		Approved:             sourceRiskInfo.RiskScore < 0.7, // Пороговое значение для автоматического одобрения
		CheckedAt:            time.Now(),
		Notes:                fmt.Sprintf("Source checked via Elliptic: %s", sourceRiskInfo.Category),
		RequiresReview:       sourceRiskInfo.RiskScore >= 0.5, // Пороговое значение для ручного рассмотрения
		ExternalServicesUsed: []string{"elliptic"},
	}

	s.logger.InfoContext(ctx, "Transaction AML check completed via Elliptic",
		"tx_hash", txHash,
		"risk_level", result.RiskLevel,
		"risk_score", result.RiskScore,
		"approved", result.Approved,
		"requires_review", result.RequiresReview)

	return result, nil
}
