package entities

type Order struct {
	ID     int    `json:"id"`
	UserID string `json:"user_id"`
	Wallet string `json:"wallet"`
	Amount string `json:"amount"`
	Status string `json:"status"`
}
