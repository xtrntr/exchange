package db

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xtrntr/exchange/internal/models"
)

var testDB *DB

func TestMain(m *testing.M) {
	pool, err := pgxpool.New(context.Background(), "postgres://exchange_user:exchange_pass@localhost:5432/exchange_db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Apply migration if not already applied
	migration, err := os.ReadFile("../../migrations/001_init.sql")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read migration: %v\n", err)
		os.Exit(1)
	}
	_, err = pool.Exec(context.Background(), string(migration))
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		fmt.Fprintf(os.Stderr, "Unable to apply migration: %v\n", err)
		os.Exit(1)
	}

	testDB = &DB{Pool: pool}
	// Truncate tables before running tests
	_, err = pool.Exec(context.Background(), "TRUNCATE TABLE users, orders, trades RESTART IDENTITY")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to truncate tables: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestDB_CreateOrder(t *testing.T) {
	// Pre-populate a user
	testDB.Pool.Exec(context.Background(), "INSERT INTO users (username, password_hash) VALUES ('alice', 'hash')")

	tests := []struct {
		name        string
		order       *models.Order
		expectError bool
	}{
		{
			name: "Success",
			order: &models.Order{
				UserID:   1,
				Type:     "sell",
				Price:    50000,
				Quantity: 0.1,
				Status:   "open",
			},
			expectError: false,
		},
		{
			name: "InvalidType",
			order: &models.Order{
				UserID:   1,
				Type:     "invalid",
				Price:    50000,
				Quantity: 0.1,
				Status:   "open",
			},
			expectError: true,
		},
		{
			name: "NegativePrice",
			order: &models.Order{
				UserID:   1,
				Type:     "sell",
				Price:    -50000,
				Quantity: 0.1,
				Status:   "open",
			},
			expectError: true,
		},
		{
			name: "ZeroQuantity",
			order: &models.Order{
				UserID:   1,
				Type:     "sell",
				Price:    50000,
				Quantity: 0,
				Status:   "open",
			},
			expectError: true,
		},
		{
			name: "NonExistentUser",
			order: &models.Order{
				UserID:   999,
				Type:     "sell",
				Price:    50000,
				Quantity: 0.1,
				Status:   "open",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset DB state
			testDB.Pool.Exec(context.Background(), "TRUNCATE TABLE orders RESTART IDENTITY")

			_, err := testDB.CreateOrder(context.Background(), tt.order)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			var count int
			err = testDB.Pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM orders WHERE user_id=1").Scan(&count)
			if err != nil || count != 1 {
				t.Errorf("order not stored in DB: %v, count=%d", err, count)
			}
		})
	}
}

func TestDB_CancelOrder(t *testing.T) {
	testDB.Pool.Exec(context.Background(), "INSERT INTO users (username, password_hash) VALUES ('alice', 'hash'), ('bob', 'hash')")
	testDB.Pool.Exec(context.Background(), `
		INSERT INTO orders (user_id, type, price, quantity, status) VALUES
		(1, 'sell', 50000, 0.1, 'open'),
		(2, 'buy', 51000, 0.05, 'open'),
		(1, 'sell', 49000, 0.2, 'filled'),
		(1, 'sell', 48000, 0.3, 'canceled')
	`)

	tests := []struct {
		name        string
		orderID     int
		userID      int
		expectError bool
	}{
		{
			name:        "Success",
			orderID:     1,
			userID:      1,
			expectError: false,
		},
		{
			name:        "NonExistentOrder",
			orderID:     999,
			userID:      1,
			expectError: true,
		},
		{
			name:        "WrongUser",
			orderID:     2,
			userID:      1,
			expectError: true,
		},
		{
			name:        "AlreadyFilled",
			orderID:     3,
			userID:      1,
			expectError: true,
		},
		{
			name:        "AlreadyCanceled",
			orderID:     4,
			userID:      1,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := testDB.CancelOrder(context.Background(), tt.orderID, tt.userID)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			var status string
			err = testDB.Pool.QueryRow(context.Background(), "SELECT status FROM orders WHERE id=$1", tt.orderID).Scan(&status)
			if err != nil || status != "canceled" {
				t.Errorf("order %d not canceled: status=%s, err=%v", tt.orderID, status, err)
			}
		})
	}
}

func TestDB_CancelOrder_Concurrent(t *testing.T) {
	// Clean up before test
	_, err := testDB.Pool.Exec(context.Background(), "TRUNCATE TABLE users, orders, trades RESTART IDENTITY")
	if err != nil {
		t.Fatalf("Failed to clean up database: %v", err)
	}

	// Insert test data
	_, err = testDB.Pool.Exec(context.Background(), "INSERT INTO users (username, password_hash) VALUES ('alice', 'hash')")
	if err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}

	_, err = testDB.Pool.Exec(context.Background(), "INSERT INTO orders (user_id, type, price, quantity, status) VALUES (1, 'sell', 50000, 0.1, 'open')")
	if err != nil {
		t.Fatalf("Failed to insert order: %v", err)
	}

	var wg sync.WaitGroup
	n := 10
	wg.Add(n)
	successCount := 0
	mu := sync.Mutex{}

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			err := testDB.CancelOrder(context.Background(), 1, 1)
			if err == nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 successful cancellation, got %d", successCount)
	}

	var status string
	err = testDB.Pool.QueryRow(context.Background(), "SELECT status FROM orders WHERE id=1").Scan(&status)
	if err != nil || status != "canceled" {
		t.Errorf("order 1 not canceled: status=%s, err=%v", status, err)
	}
}

func TestDB_GetUserOrders(t *testing.T) {
	// Clean up before test
	_, err := testDB.Pool.Exec(context.Background(), "TRUNCATE TABLE users, orders, trades RESTART IDENTITY")
	if err != nil {
		t.Fatalf("Failed to clean up database: %v", err)
	}

	// Insert test data
	_, err = testDB.Pool.Exec(context.Background(), "INSERT INTO users (username, password_hash) VALUES ('alice', 'hash'), ('bob', 'hash')")
	if err != nil {
		t.Fatalf("Failed to insert users: %v", err)
	}

	_, err = testDB.Pool.Exec(context.Background(), `
		INSERT INTO orders (user_id, type, price, quantity, status) VALUES
		(1, 'sell', 50000, 0.1, 'open'),
		(1, 'buy', 49000, 0.2, 'filled'),
		(1, 'sell', 48000, 0.3, 'canceled'),
		(2, 'buy', 51000, 0.05, 'open')
	`)
	if err != nil {
		t.Fatalf("Failed to insert orders: %v", err)
	}

	tests := []struct {
		name         string
		userID       int
		expectCount  int
		expectStatus []string
	}{
		{
			name:         "UserWithOrders",
			userID:       1,
			expectCount:  3,
			expectStatus: []string{"open", "filled", "canceled"},
		},
		{
			name:         "UserWithOneOrder",
			userID:       2,
			expectCount:  1,
			expectStatus: []string{"open"},
		},
		{
			name:         "UserWithNoOrders",
			userID:       999,
			expectCount:  0,
			expectStatus: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orders, err := testDB.GetUserOrders(context.Background(), tt.userID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(orders) != tt.expectCount {
				t.Errorf("expected %d orders, got %d", tt.expectCount, len(orders))
			}

			for i, status := range tt.expectStatus {
				if i < len(orders) && orders[i].Status != status {
					t.Errorf("expected status %s, got %s", status, orders[i].Status)
				}
			}
		})
	}
}
