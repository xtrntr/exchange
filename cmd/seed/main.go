package main

import (
	"context"
	"fmt"
	"log"
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

	// Create buy orders for user1
	var buyOrder1, buyOrder2, buyOrder3 int
	err = database.Pool.QueryRow(ctx,
		"INSERT INTO orders (user_id, type, price, quantity, status, created_at) VALUES ($1, 'buy', 30000, 0.1, 'filled', NOW() - INTERVAL '3 day') RETURNING id",
		user1ID).Scan(&buyOrder1)
	if err != nil {
		log.Fatalf("Failed to create buy order 1: %v", err)
	}

	err = database.Pool.QueryRow(ctx,
		"INSERT INTO orders (user_id, type, price, quantity, status, created_at) VALUES ($1, 'buy', 31000, 0.2, 'filled', NOW() - INTERVAL '2 day') RETURNING id",
		user1ID).Scan(&buyOrder2)
	if err != nil {
		log.Fatalf("Failed to create buy order 2: %v", err)
	}

	err = database.Pool.QueryRow(ctx,
		"INSERT INTO orders (user_id, type, price, quantity, status, created_at) VALUES ($1, 'buy', 32000, 0.15, 'filled', NOW() - INTERVAL '1 day') RETURNING id",
		user1ID).Scan(&buyOrder3)
	if err != nil {
		log.Fatalf("Failed to create buy order 3: %v", err)
	}

	// Create sell orders for user2
	var sellOrder1, sellOrder2, sellOrder3 int
	err = database.Pool.QueryRow(ctx,
		"INSERT INTO orders (user_id, type, price, quantity, status, created_at) VALUES ($1, 'sell', 30000, 0.1, 'filled', NOW() - INTERVAL '3 day') RETURNING id",
		user2ID).Scan(&sellOrder1)
	if err != nil {
		log.Fatalf("Failed to create sell order 1: %v", err)
	}

	err = database.Pool.QueryRow(ctx,
		"INSERT INTO orders (user_id, type, price, quantity, status, created_at) VALUES ($1, 'sell', 31000, 0.2, 'filled', NOW() - INTERVAL '2 day') RETURNING id",
		user2ID).Scan(&sellOrder2)
	if err != nil {
		log.Fatalf("Failed to create sell order 2: %v", err)
	}

	err = database.Pool.QueryRow(ctx,
		"INSERT INTO orders (user_id, type, price, quantity, status, created_at) VALUES ($1, 'sell', 32000, 0.15, 'filled', NOW() - INTERVAL '1 day') RETURNING id",
		user2ID).Scan(&sellOrder3)
	if err != nil {
		log.Fatalf("Failed to create sell order 3: %v", err)
	}

	// Create trades between the orders
	baseTime := time.Now().Add(-3 * 24 * time.Hour) // 3 days ago

	// Create trade 1 (3 days ago)
	trade1 := models.Trade{
		BuyOrderID:  buyOrder1,
		SellOrderID: sellOrder1,
		Price:       30000,
		Quantity:    0.1,
		ExecutedAt:  baseTime,
	}
	_, err = database.CreateTrade(ctx, &trade1)
	if err != nil {
		log.Fatalf("Failed to create trade 1: %v", err)
	}

	// Create trade 2 (2 days ago)
	trade2 := models.Trade{
		BuyOrderID:  buyOrder2,
		SellOrderID: sellOrder2,
		Price:       31000,
		Quantity:    0.2,
		ExecutedAt:  baseTime.Add(24 * time.Hour),
	}
	_, err = database.CreateTrade(ctx, &trade2)
	if err != nil {
		log.Fatalf("Failed to create trade 2: %v", err)
	}

	// Create trade 3 (1 day ago)
	trade3 := models.Trade{
		BuyOrderID:  buyOrder3,
		SellOrderID: sellOrder3,
		Price:       32000,
		Quantity:    0.15,
		ExecutedAt:  baseTime.Add(48 * time.Hour),
	}
	_, err = database.CreateTrade(ctx, &trade3)
	if err != nil {
		log.Fatalf("Failed to create trade 3: %v", err)
	}

	fmt.Println("Successfully seeded the database with test trades!")
}
