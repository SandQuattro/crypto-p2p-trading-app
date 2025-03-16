package entities

import (
	"time"
)

// Wallet represents a tracked wallet in our system
type Wallet struct {
	ID             int       `db:"id"`
	UserID         int       `db:"user_id"`
	Address        string    `db:"address"`
	DerivationPath string    `db:"derivation_path"`
	WalletIndex    uint32    `db:"wallet_index"`
	CreatedAt      time.Time `db:"created_at"`
}
