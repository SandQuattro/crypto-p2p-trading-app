package entities

import "time"

// RiskLevel представляет уровень риска транзакции
type RiskLevel string

const (
	RiskLevelLow    RiskLevel = "low"
	RiskLevelMedium RiskLevel = "medium"
	RiskLevelHigh   RiskLevel = "high"
)

// RiskSource представляет источник информации о риске
type RiskSource string

const (
	RiskSourceSanctionsList RiskSource = "sanctions_list"
	RiskSourceBehavioral    RiskSource = "behavioral"
	RiskSourceMLDetection   RiskSource = "ml_detection"
	RiskSourceTaintedFunds  RiskSource = "tainted_funds"
)

// AMLCheckResult содержит результат AML проверки транзакции
type AMLCheckResult struct {
	ID                   int        `json:"id"`
	TransactionHash      string     `json:"transaction_hash"`
	WalletAddress        string     `json:"wallet_address"`
	SourceAddress        string     `json:"source_address"`
	RiskLevel            RiskLevel  `json:"risk_level"`
	RiskSource           RiskSource `json:"risk_source"`
	RiskScore            float64    `json:"risk_score"`
	Approved             bool       `json:"approved"`
	CheckedAt            time.Time  `json:"checked_at"`
	Notes                string     `json:"notes,omitempty"`
	RequiresReview       bool       `json:"requires_review"`
	ExternalServicesUsed []string   `json:"external_services_used,omitempty"`
}

// AddressRiskInfo содержит информацию о риске, связанном с адресом
type AddressRiskInfo struct {
	Address     string    `json:"address"`
	RiskLevel   RiskLevel `json:"risk_level"`
	RiskScore   float64   `json:"risk_score"`
	LastChecked time.Time `json:"last_checked"`
	Category    string    `json:"category,omitempty"`
	Source      string    `json:"source,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
}

// TransactionCheck представляет информацию о необходимости проверки транзакции
type TransactionCheck struct {
	TxHash        string    `json:"tx_hash"`
	WalletAddress string    `json:"wallet_address"`
	SourceAddress string    `json:"source_address"`
	Amount        string    `json:"amount"`
	CreatedAt     time.Time `json:"created_at"`
	Processed     bool      `json:"processed"`
}
