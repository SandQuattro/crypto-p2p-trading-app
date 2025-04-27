package entities

import (
	"math/big"
	"time"
)

// Wallet represents a tracked wallet in our system
type Wallet struct {
	ID             int       `db:"id"`
	UserID         int64     `db:"user_id"`
	Address        string    `db:"address"`
	DerivationPath string    `db:"derivation_path"`
	WalletIndex    uint32    `db:"wallet_index"`
	IsTestnet      bool      `db:"is_testnet"`
	CreatedAt      time.Time `db:"created_at"`
}

// WalletDetail represents wallet information with ID and address
type WalletDetail struct {
	ID      int64  `json:"id"`
	Address string `json:"address"`
}

// WalletDetailExtended represents wallet information with ID, address, user ID and creation date
type WalletDetailExtended struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Address   string    `json:"address"`
	IsTestnet bool      `json:"is_testnet"`
	CreatedAt time.Time `json:"created_at"`
}

// BalanceStatus represents the status of a wallet balance
type BalanceStatus string

const (
	// BalanceStatusOK indicates that the wallet balance is sufficient
	BalanceStatusOK BalanceStatus = "ok"

	// BalanceStatusLow indicates that the wallet balance is getting low
	BalanceStatusLow BalanceStatus = "low"

	// BalanceStatusCritical indicates that the wallet balance is critically low
	BalanceStatusCritical BalanceStatus = "critical"
)

// WalletBalance represents balance information for a wallet
type WalletBalance struct {
	Address       string        `json:"address"`
	TokenBalance  *big.Int      `json:"token_balance"`
	NativeBalance *big.Int      `json:"native_balance"` // BNB balance
	Status        BalanceStatus `json:"status"`
	LastChecked   time.Time     `json:"last_checked"`
}
