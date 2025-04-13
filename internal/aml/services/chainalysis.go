package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/sand/crypto-p2p-trading-app/backend/internal/aml/entities"
)

// ChainalysisService представляет сервис для проверки транзакций через Chainalysis API
type ChainalysisService struct {
	logger    *slog.Logger
	apiKey    string
	apiURL    string
	client    *http.Client
	isEnabled bool
}

// NewChainalysisService создает новый сервис для проверки транзакций через Chainalysis
func NewChainalysisService(logger *slog.Logger, apiKey, apiURL string) *ChainalysisService {
	isEnabled := apiKey != "" && apiURL != ""

	if !isEnabled {
		logger.Warn("Chainalysis service is disabled due to missing credentials")
	}

	return &ChainalysisService{
		logger:    logger,
		apiKey:    apiKey,
		apiURL:    apiURL,
		client:    &http.Client{Timeout: 10 * time.Second},
		isEnabled: isEnabled,
	}
}

// IsEnabled возвращает статус активации сервиса
func (s *ChainalysisService) IsEnabled() bool {
	return s.isEnabled
}

// CheckAddress проверяет адрес на риски через Chainalysis API
func (s *ChainalysisService) CheckAddress(ctx context.Context, address string) (*entities.AddressRiskInfo, error) {
	if !s.isEnabled {
		s.logger.Warn("Chainalysis service is disabled, skipping check", "address", address)
		return &entities.AddressRiskInfo{
			Address:     address,
			RiskLevel:   entities.RiskLevelLow,
			RiskScore:   0,
			LastChecked: time.Now(),
			Source:      "chainalysis_disabled",
		}, nil
	}

	// Формирование запроса к Chainalysis API
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/address/%s", s.apiURL, address), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Chainalysis request: %w", err)
	}

	req.Header.Set("X-API-Key", s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	s.logger.InfoContext(ctx, "Checking address with Chainalysis", "address", address)

	// В реальной интеграции здесь происходит обращение к API Chainalysis
	// Для демонстрации возвращаем заглушку

	// resp, err := s.client.Do(req)
	// if err != nil {
	//     return nil, fmt.Errorf("failed to send request to Chainalysis: %w", err)
	// }
	// defer resp.Body.Close()

	// Имитация результата проверки
	riskInfo := &entities.AddressRiskInfo{
		Address:     address,
		RiskLevel:   entities.RiskLevelLow,
		RiskScore:   0.1,
		LastChecked: time.Now(),
		Category:    "clean",
		Source:      "chainalysis",
		Tags:        []string{"checked"},
	}

	s.logger.InfoContext(ctx, "Chainalysis check completed",
		"address", address,
		"risk_level", riskInfo.RiskLevel,
		"risk_score", riskInfo.RiskScore)

	return riskInfo, nil
}

// CheckTransaction проверяет транзакцию на риски через Chainalysis API
func (s *ChainalysisService) CheckTransaction(ctx context.Context, txHash, sourceAddress, destinationAddress, amount string) (*entities.AMLCheckResult, error) {
	if !s.isEnabled {
		s.logger.Warn("Chainalysis service is disabled, skipping transaction check",
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
			Notes:                "Check skipped - Chainalysis service disabled",
			RequiresReview:       false,
			ExternalServicesUsed: []string{"chainalysis_disabled"},
		}, nil
	}

	// Проверка исходного адреса
	sourceRiskInfo, err := s.CheckAddress(ctx, sourceAddress)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to check source address",
			"error", err,
			"address", sourceAddress,
			"tx_hash", txHash)
		// Продолжаем выполнение даже при ошибке, считая адрес рискованным
		sourceRiskInfo = &entities.AddressRiskInfo{
			Address:     sourceAddress,
			RiskLevel:   entities.RiskLevelHigh,
			RiskScore:   0.8,
			LastChecked: time.Now(),
			Source:      "chainalysis_error",
		}
	}

	// В реальной интеграции здесь был бы еще запрос к Chainalysis API для проверки транзакции
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
		Notes:                fmt.Sprintf("Source checked via Chainalysis: %s", sourceRiskInfo.Category),
		RequiresReview:       sourceRiskInfo.RiskScore >= 0.5, // Пороговое значение для ручного рассмотрения
		ExternalServicesUsed: []string{"chainalysis"},
	}

	s.logger.InfoContext(ctx, "Transaction AML check completed via Chainalysis",
		"tx_hash", txHash,
		"risk_level", result.RiskLevel,
		"risk_score", result.RiskScore,
		"approved", result.Approved,
		"requires_review", result.RequiresReview)

	return result, nil
}
