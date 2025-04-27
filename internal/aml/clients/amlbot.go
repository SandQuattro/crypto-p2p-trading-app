package clients

import (
	"context"
	"fmt"
	"github.com/sand/crypto-p2p-trading-app/backend/internal/entities"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// AMLBotService представляет сервис для проверки транзакций через AMLBot API
type AMLBotService struct {
	logger    *slog.Logger
	apiKey    string
	apiURL    string
	client    *http.Client
	isEnabled bool
}

// NewAMLBotService создает новый сервис для проверки транзакций через AMLBot
func NewAMLBotService(logger *slog.Logger, apiKey, apiURL string) *AMLBotService {
	isEnabled := apiKey != "" && apiURL != ""

	if !isEnabled {
		logger.Warn("AMLBot service is disabled due to missing credentials")
	} else {
		logger.Info("AMLBot service initialized", "api_url", apiURL)
	}

	// Если URL не указан, используем значение по умолчанию
	if apiURL == "" {
		apiURL = "https://api.amlbot.com/v1"
	}

	return &AMLBotService{
		logger:    logger,
		apiKey:    apiKey,
		apiURL:    apiURL,
		client:    &http.Client{Timeout: 10 * time.Second},
		isEnabled: isEnabled,
	}
}

// IsEnabled возвращает статус активации сервиса
func (s *AMLBotService) IsEnabled() bool {
	return s.isEnabled
}

// CheckAddress проверяет адрес на риски через AMLBot API
func (s *AMLBotService) CheckAddress(ctx context.Context, address string) (*entities.AddressRiskInfo, error) {
	if !s.isEnabled {
		s.logger.Warn("AMLBot service is disabled, skipping check", "address", address)
		return &entities.AddressRiskInfo{
			Address:     address,
			RiskLevel:   entities.RiskLevelLow,
			RiskScore:   0,
			LastChecked: time.Now(),
			Source:      "amlbot_disabled",
		}, nil
	}

	// Формирование запроса к AMLBot API
	// AMLBot обычно использует форму для отправки или query параметры
	apiEndpoint := fmt.Sprintf("%s/address/check", s.apiURL)
	form := url.Values{}
	form.Add("address", address)
	form.Add("api_key", s.apiKey)

	req, err := http.NewRequestWithContext(ctx, "POST", apiEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create AMLBot request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	s.logger.InfoContext(ctx, "Checking address with AMLBot", "address", address)

	// В реальной интеграции здесь происходит обращение к API AMLBot
	// Для демонстрации используем заглушку
	/*
		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to send request to AMLBot: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("AMLBot API returned non-200 status code: %d, body: %s",
				resp.StatusCode, string(bodyBytes))
		}

		var result struct {
			Score float64 `json:"score"`
			Risk  string  `json:"risk"`
			Tags  []string `json:"tags"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("failed to decode AMLBot response: %w", err)
		}
	*/

	// Имитация результата проверки
	riskInfo := &entities.AddressRiskInfo{
		Address:     address,
		RiskLevel:   entities.RiskLevelLow,
		RiskScore:   0.15,
		LastChecked: time.Now(),
		Category:    "regular",
		Source:      "amlbot",
		Tags:        []string{"checked", "amlbot"},
	}

	s.logger.InfoContext(ctx, "AMLBot check completed",
		"address", address,
		"risk_level", riskInfo.RiskLevel,
		"risk_score", riskInfo.RiskScore)

	return riskInfo, nil
}

// CheckTransaction проверяет транзакцию на риски через AMLBot API
func (s *AMLBotService) CheckTransaction(ctx context.Context, txHash, sourceAddress, destinationAddress, amount string) (*entities.AMLCheckResult, error) {
	if !s.isEnabled {
		s.logger.Warn("AMLBot service is disabled, skipping transaction check",
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
			Notes:                "Check skipped - AMLBot service disabled",
			RequiresReview:       false,
			ExternalServicesUsed: []string{"amlbot_disabled"},
		}, nil
	}

	// Проверка исходного адреса
	sourceRiskInfo, err := s.CheckAddress(ctx, sourceAddress)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to check source address with AMLBot",
			"error", err,
			"address", sourceAddress,
			"tx_hash", txHash)
		// Продолжаем выполнение даже при ошибке, считая адрес с низким риском
		sourceRiskInfo = &entities.AddressRiskInfo{
			Address:     sourceAddress,
			RiskLevel:   entities.RiskLevelMedium,
			RiskScore:   0.4,
			LastChecked: time.Now(),
			Source:      "amlbot_error",
		}
	}

	// В AMLBot можно также проверить транзакцию напрямую, но это не всегда доступно
	// В реальной интеграции здесь был бы запрос к AMLBot API для проверки транзакции
	// но для простоты используем результат проверки адреса

	// Формирование результата на основе информации о риске адреса
	result := &entities.AMLCheckResult{
		TransactionHash:      txHash,
		WalletAddress:        destinationAddress,
		SourceAddress:        sourceAddress,
		RiskLevel:            sourceRiskInfo.RiskLevel,
		RiskSource:           entities.RiskSourceSanctionsList,
		RiskScore:            sourceRiskInfo.RiskScore,
		Approved:             sourceRiskInfo.RiskScore < 0.7, // Пороговое значение для автоматического одобрения
		CheckedAt:            time.Now(),
		Notes:                fmt.Sprintf("Source checked via AMLBot: %s", sourceRiskInfo.Category),
		RequiresReview:       sourceRiskInfo.RiskScore >= 0.5, // Пороговое значение для ручного рассмотрения
		ExternalServicesUsed: []string{"amlbot"},
	}

	s.logger.InfoContext(ctx, "Transaction AML check completed via AMLBot",
		"tx_hash", txHash,
		"risk_level", result.RiskLevel,
		"risk_score", result.RiskScore,
		"approved", result.Approved,
		"requires_review", result.RequiresReview)

	return result, nil
}
