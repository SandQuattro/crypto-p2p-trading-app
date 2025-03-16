package entities

type Order struct {
	ID       int    `json:"id"`
	UserID   int    `json:"user_id"`
	WalletID int    `json:"wallet_id"`
	Amount   string `json:"amount"`
	Status   string `json:"status"`
}
