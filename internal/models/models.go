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
	ID           int
	BuyOrderID   int
	SellOrderID  int
	Price        float64
	Quantity     float64
	ExecutedAt   time.Time
} 