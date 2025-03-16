package entities

import "time"

// Transaction represents a blockchain transaction in our system.
type Transaction struct {
	ID            int       `json:"id"`
	TxHash        string    `json:"tx_hash"`
	WalletAddress string    `json:"wallet_address"`
	Amount        string    `json:"amount"`
	BlockNumber   int64     `json:"block_number"`
	Confirmed     bool      `json:"confirmed"`
	Processed     bool      `json:"processed"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ConfirmedUnprocessedTransaction struct {
	Id            int
	TxHash        string
	WalletAddress string
	Amount        string
}
