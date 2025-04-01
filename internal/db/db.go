package db

import (
	"context"
	"fmt"

	"github.com/xtrntr/exchange/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a PostgreSQL connection pool
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB initializes a new database connection pool
func NewDB(ctx context.Context, connString string) (*DB, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Close closes the database connection pool
func (db *DB) Close(ctx context.Context) error {
	db.Pool.Close()
	return nil
}

// CreateUser inserts a new user
func (db *DB) CreateUser(ctx context.Context, username, passwordHash string) (*models.User, error) {
	user := &models.User{}
	err := db.Pool.QueryRow(ctx,
		"INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id, username, password_hash, created_at",
		username, passwordHash).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return user, nil
}

// GetUserByUsername retrieves a user by username
func (db *DB) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	user := &models.User{}
	err := db.Pool.QueryRow(ctx,
		"SELECT id, username, password_hash, created_at FROM users WHERE username = $1",
		username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// CreateOrder inserts a new order
func (db *DB) CreateOrder(ctx context.Context, order *models.Order) (*models.Order, error) {
	// Validate order
	if order.Type != "buy" && order.Type != "sell" {
		return nil, fmt.Errorf("type must be 'buy' or 'sell'")
	}
	if order.Price <= 0 {
		return nil, fmt.Errorf("price must be positive")
	}
	if order.Quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}

	// Verify user exists
	var exists bool
	err := db.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", order.UserID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check user existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	newOrder := &models.Order{}
	err = db.Pool.QueryRow(ctx,
		"INSERT INTO orders (user_id, type, price, quantity, status) VALUES ($1, $2, $3, $4, $5) RETURNING id, user_id, type, price, quantity, status, created_at",
		order.UserID, order.Type, order.Price, order.Quantity, order.Status).Scan(
		&newOrder.ID, &newOrder.UserID, &newOrder.Type, &newOrder.Price, &newOrder.Quantity, &newOrder.Status, &newOrder.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}
	return newOrder, nil
}

// UpdateOrderStatus updates an order's status
func (db *DB) UpdateOrderStatus(ctx context.Context, orderID int, status string) error {
	_, err := db.Pool.Exec(ctx, "UPDATE orders SET status = $1 WHERE id = $2", status, orderID)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	return nil
}

// GetUserOrders retrieves all orders for a user
func (db *DB) GetUserOrders(ctx context.Context, userID int) ([]models.Order, error) {
	rows, err := db.Pool.Query(ctx,
		"SELECT id, user_id, type, price, quantity, status, created_at FROM orders WHERE user_id = $1",
		userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user orders: %w", err)
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var order models.Order
		if err := rows.Scan(&order.ID, &order.UserID, &order.Type, &order.Price, &order.Quantity, &order.Status, &order.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}
	return orders, nil
}

// CreateTrade inserts a new trade
func (db *DB) CreateTrade(ctx context.Context, trade *models.Trade) (*models.Trade, error) {
	newTrade := &models.Trade{}
	err := db.Pool.QueryRow(ctx,
		"INSERT INTO trades (buy_order_id, sell_order_id, price, quantity) VALUES ($1, $2, $3, $4) RETURNING id, buy_order_id, sell_order_id, price, quantity, executed_at",
		trade.BuyOrderID, trade.SellOrderID, trade.Price, trade.Quantity).Scan(
		&newTrade.ID, &newTrade.BuyOrderID, &newTrade.SellOrderID, &newTrade.Price, &newTrade.Quantity, &newTrade.ExecutedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create trade: %w", err)
	}
	return newTrade, nil
}

// GetUserTrades retrieves all trades for a user
func (db *DB) GetUserTrades(ctx context.Context, userID int) ([]models.Trade, error) {
	rows, err := db.Pool.Query(ctx,
		"SELECT t.id, t.buy_order_id, t.sell_order_id, t.price, t.quantity, t.executed_at "+
			"FROM trades t JOIN orders o ON t.buy_order_id = o.id OR t.sell_order_id = o.id "+
			"WHERE o.user_id = $1",
		userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user trades: %w", err)
	}
	defer rows.Close()

	var trades []models.Trade
	for rows.Next() {
		var trade models.Trade
		if err := rows.Scan(&trade.ID, &trade.BuyOrderID, &trade.SellOrderID, &trade.Price, &trade.Quantity, &trade.ExecutedAt); err != nil {
			return nil, fmt.Errorf("failed to scan trade: %w", err)
		}
		trades = append(trades, trade)
	}
	return trades, nil
}

// CancelOrder cancels an order if it belongs to the user and is open
func (db *DB) CancelOrder(ctx context.Context, orderID, userID int) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock the row for update to prevent concurrent modifications
	var status string
	err = tx.QueryRow(ctx,
		"SELECT status FROM orders WHERE id = $1 AND user_id = $2 FOR UPDATE",
		orderID, userID).Scan(&status)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("order not found or not owned by user")
		}
		return fmt.Errorf("failed to get order: %w", err)
	}

	if status != "open" {
		return fmt.Errorf("order not open")
	}

	tag, err := tx.Exec(ctx,
		"UPDATE orders SET status = 'canceled' WHERE id = $1 AND user_id = $2 AND status = 'open'",
		orderID, userID)
	if err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("order not found, not owned by user, or not open")
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetOpenOrders retrieves all open orders from the database
func (db *DB) GetOpenOrders(ctx context.Context) ([]models.Order, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, user_id, type, price, quantity, status, created_at
		FROM orders
		WHERE status = 'open'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var order models.Order
		err := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.Type,
			&order.Price,
			&order.Quantity,
			&order.Status,
			&order.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}
