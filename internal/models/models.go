package models

import "time"

// User represents a registered user
type User struct {
	ID           int
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

// Order represents a buy or sell order
type Order struct {
	ID        int
	UserID    int
	Type      string    // "buy" or "sell"
	Price     float64   // Price in USD
	Quantity  float64   // Quantity in BTC
	Status    string    // "open", "filled", "canceled"
	CreatedAt time.Time // Used for time priority
}

// Trade represents an executed trade
type Trade struct {
	ID          int       `json:"id"`
	BuyOrderID  int       `json:"buy_order_id"`
	SellOrderID int       `json:"sell_order_id"`
	Price       float64   `json:"price"`
	Quantity    float64   `json:"quantity"`
	ExecutedAt  time.Time `json:"executed_at"`
}
