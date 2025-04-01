package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	dbConnString = "postgres://exchange_user:exchange_pass@localhost:5432/exchange_db"
)

func main() {
	ctx := context.Background()

	// Connect to database
	conn, err := pgx.Connect(ctx, dbConnString)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	// Clean up existing data
	fmt.Println("Cleaning up existing data...")
	_, err = conn.Exec(ctx, `
		DELETE FROM trades;
		DELETE FROM orders;
		DELETE FROM users;
	`)
	if err != nil {
		log.Fatalf("Failed to clean up existing data: %v", err)
	}

	// Create users
	users := []struct {
		username string
		password string
	}{
		{"testuser1", "pass1"},
		{"testuser2", "pass2"},
	}

	userIDs := make([]int64, len(users))
	for i, user := range users {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.password), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("Failed to hash password: %v", err)
		}

		err = conn.QueryRow(ctx,
			"INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id",
			user.username, hashedPassword).Scan(&userIDs[i])
		if err != nil {
			log.Fatalf("Failed to insert user: %v", err)
		}
	}

	// Create buy orders
	buyPrices := []float64{30000, 30200, 30400, 30600, 30800}
	buyQuantities := []float64{0.5, 0.4, 0.3, 0.2, 0.1}
	buyOrderIDs := make([]int64, len(buyPrices))

	baseTime := time.Now().Add(-10 * time.Minute)
	for i := range buyPrices {
		err = conn.QueryRow(ctx,
			"INSERT INTO orders (user_id, type, price, quantity, status, created_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
			userIDs[0], "buy", buyPrices[i], buyQuantities[i], "open",
			baseTime.Add(time.Duration(i)*time.Minute)).Scan(&buyOrderIDs[i])
		if err != nil {
			log.Fatalf("Failed to insert buy order: %v", err)
		}
	}

	// Create sell orders
	sellPrices := []float64{31000, 31200, 31400, 31600, 31800}
	sellQuantities := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	sellOrderIDs := make([]int64, len(sellPrices))

	for i := range sellPrices {
		err = conn.QueryRow(ctx,
			"INSERT INTO orders (user_id, type, price, quantity, status, created_at) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
			userIDs[1], "sell", sellPrices[i], sellQuantities[i], "open",
			baseTime.Add(time.Duration(i)*time.Minute)).Scan(&sellOrderIDs[i])
		if err != nil {
			log.Fatalf("Failed to insert sell order: %v", err)
		}
	}

	// Create trades (only for first few orders)
	tradePrices := []float64{31000, 31010}   // Only 2 trades
	tradeQuantities := []float64{0.05, 0.06} // Only 2 quantities

	for i := range tradePrices {
		_, err = conn.Exec(ctx,
			"INSERT INTO trades (buy_order_id, sell_order_id, price, quantity, executed_at) VALUES ($1, $2, $3, $4, $5)",
			buyOrderIDs[i], sellOrderIDs[i], tradePrices[i], tradeQuantities[i],
			baseTime.Add(time.Duration(i)*time.Minute))
		if err != nil {
			log.Fatalf("Failed to insert trade: %v", err)
		}

		// Update order status to filled only for traded orders
		_, err = conn.Exec(ctx,
			"UPDATE orders SET status = 'filled' WHERE id IN ($1, $2)",
			buyOrderIDs[i], sellOrderIDs[i])
		if err != nil {
			log.Fatalf("Failed to update order status: %v", err)
		}
	}

	fmt.Println("Successfully seeded database with test data!")
}
