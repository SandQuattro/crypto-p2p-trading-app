package entities

import "time"

// AMLStatus представляет статус AML проверки транзакции
type AMLStatus string

const (
	AMLStatusNone    AMLStatus = "none"    // Проверка не проводилась или нет подозрений
	AMLStatusFlagged AMLStatus = "flagged" // Помечена как подозрительная
	AMLStatusCleared AMLStatus = "cleared" // Проверена вручную и одобрена
)

// Transaction represents a blockchain transaction in our system.
type Transaction struct {
	ID            int       `json:"id"`
	TxHash        string    `json:"tx_hash"`
	WalletAddress string    `json:"wallet_address"`
	Amount        string    `json:"amount"`
	BlockNumber   int64     `json:"block_number"`
	Confirmed     bool      `json:"confirmed"`
	Processed     bool      `json:"processed"`
	AMLStatus     AMLStatus `json:"aml_status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ConfirmedUnprocessedTransaction struct {
	Id            int
	TxHash        string
	WalletAddress string
	Amount        string
}
