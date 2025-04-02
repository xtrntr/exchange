package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/xtrntr/exchange/internal/db"
	"github.com/xtrntr/exchange/internal/models"
)

// Seed the database with test data
func main() {
	ctx := context.Background()

	// Connect to database
	connString := "postgres://exchange_user:exchange_pass@localhost:5432/exchange_db?sslmode=disable"
	database, err := db.NewDB(ctx, connString)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close(ctx)

	// First check if we already have trades
	trades, err := database.GetAllTrades(ctx)
	if err != nil {
		log.Fatalf("Failed to check trades: %v", err)
	}

	if len(trades) > 0 {
		fmt.Printf("Database already has %d trades. No need to seed.\n", len(trades))
		os.Exit(0)
	}

	// Create test users if they don't exist
	var user1ID, user2ID int
	err = database.Pool.QueryRow(ctx, "SELECT id FROM users WHERE username = 'trader1'").Scan(&user1ID)
	if err != nil {
		// Create user1
		_, err = database.Pool.Exec(ctx,
			"INSERT INTO users (username, password_hash) VALUES ('trader1', '$2a$10$XLhV7TU4dIvHO1d9UKgoT.Kt1XCYIbLV4LkQqmXGtN6VBnsmgS.G.') RETURNING id")
		if err != nil {
			log.Fatalf("Failed to create user1: %v", err)
		}
		err = database.Pool.QueryRow(ctx, "SELECT id FROM users WHERE username = 'trader1'").Scan(&user1ID)
		if err != nil {
			log.Fatalf("Failed to get user1 ID: %v", err)
		}
	}

	err = database.Pool.QueryRow(ctx, "SELECT id FROM users WHERE username = 'trader2'").Scan(&user2ID)
	if err != nil {
		// Create user2
		_, err = database.Pool.Exec(ctx,
			"INSERT INTO users (username, password_hash) VALUES ('trader2', '$2a$10$XLhV7TU4dIvHO1d9UKgoT.Kt1XCYIbLV4LkQqmXGtN6VBnsmgS.G.') RETURNING id")
		if err != nil {
			log.Fatalf("Failed to create user2: %v", err)
		}
		err = database.Pool.QueryRow(ctx, "SELECT id FROM users WHERE username = 'trader2'").Scan(&user2ID)
		if err != nil {
			log.Fatalf("Failed to get user2 ID: %v", err)
		}
	}

	// Generate trades over the last hour with 1-minute intervals
	baseTime := time.Now().Add(-1 * time.Hour)
	basePrice := 800.0     // Starting price around 800
	priceVolatility := 5.0 // Max price movement per minute (±$5)

	for i := 0; i < 60; i++ { // One hour of data
		tradeTime := baseTime.Add(time.Duration(i) * time.Minute)

		// Calculate price movement for this minute
		priceChange := (rand.Float64()*2 - 1) * priceVolatility
		currentPrice := basePrice + priceChange

		// Create 2-5 trades per minute
		numTrades := rand.Intn(4) + 2
		for j := 0; j < numTrades; j++ {
			// Add small price variation within the minute
			tradePrice := currentPrice + (rand.Float64()*2-1)*0.5 // ±$0.50 variation
			quantity := 0.1 + rand.Float64()*0.9                  // Random quantity between 0.1 and 1.0 BTC

			// Create buy order
			var buyOrderID int
			err = database.Pool.QueryRow(ctx,
				"INSERT INTO orders (user_id, type, price, quantity, status, created_at) VALUES ($1, 'buy', $2, $3, 'filled', $4) RETURNING id",
				user1ID, tradePrice, quantity, tradeTime).Scan(&buyOrderID)
			if err != nil {
				log.Fatalf("Failed to create buy order: %v", err)
			}

			// Create sell order
			var sellOrderID int
			err = database.Pool.QueryRow(ctx,
				"INSERT INTO orders (user_id, type, price, quantity, status, created_at) VALUES ($1, 'sell', $2, $3, 'filled', $4) RETURNING id",
				user2ID, tradePrice, quantity, tradeTime).Scan(&sellOrderID)
			if err != nil {
				log.Fatalf("Failed to create sell order: %v", err)
			}

			// Create trade
			trade := models.Trade{
				BuyOrderID:  buyOrderID,
				SellOrderID: sellOrderID,
				Price:       tradePrice,
				Quantity:    quantity,
				ExecutedAt:  tradeTime,
			}
			_, err = database.CreateTrade(ctx, &trade)
			if err != nil {
				log.Fatalf("Failed to create trade: %v", err)
			}
		}

		// Update base price for next minute
		basePrice += priceChange * 0.5 // Trend continuation
	}

	fmt.Println("Successfully seeded the database with test trades!")
}
