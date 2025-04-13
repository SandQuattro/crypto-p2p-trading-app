package entities

import "time"

// Order represents a user order in our system
type Order struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	WalletID  int       `json:"wallet_id"`
	Amount    string    `json:"amount"`
	Status    string    `json:"status"`
	AMLStatus AMLStatus `json:"aml_status"`
	AMLNotes  string    `json:"aml_notes,omitempty"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
