package db

import (
	"context"
	"exchange/internal/models"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// DB wraps a PostgreSQL connection
type DB struct {
	Conn *pgx.Conn
}

// NewDB initializes a new database connection
func NewDB(ctx context.Context, connString string) (*DB, error) {
	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return &DB{Conn: conn}, nil
}

// Close closes the database connection
func (db *DB) Close(ctx context.Context) error {
	return db.Conn.Close(ctx)
}

// CreateUser inserts a new user
func (db *DB) CreateUser(ctx context.Context, username, passwordHash string) (*models.User, error) {
	user := &models.User{}
	err := db.Conn.QueryRow(ctx,
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
	err := db.Conn.QueryRow(ctx,
		"SELECT id, username, password_hash, created_at FROM users WHERE username = $1",
		username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// CreateOrder inserts a new order
func (db *DB) CreateOrder(ctx context.Context, order *models.Order) (*models.Order, error) {
	newOrder := &models.Order{}
	err := db.Conn.QueryRow(ctx,
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
	_, err := db.Conn.Exec(ctx, "UPDATE orders SET status = $1 WHERE id = $2", status, orderID)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}
	return nil
}

// GetUserOrders retrieves all orders for a user
func (db *DB) GetUserOrders(ctx context.Context, userID int) ([]models.Order, error) {
	rows, err := db.Conn.Query(ctx,
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
	err := db.Conn.QueryRow(ctx,
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
	rows, err := db.Conn.Query(ctx,
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