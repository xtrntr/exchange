package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/xtrntr/exchange/internal/api"
	"github.com/xtrntr/exchange/internal/auth"
	"github.com/xtrntr/exchange/internal/db"
	"github.com/xtrntr/exchange/internal/exchange"
	"github.com/xtrntr/exchange/internal/models"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

type WSClient struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

var (
	clients   = make(map[*WSClient]bool)
	clientsMu sync.RWMutex
)

func broadcastOrderBook(ex *exchange.Exchange) {
	buyOrders, sellOrders := ex.GetOrderBook()
	orderBook := struct {
		BuyOrders  []models.Order `json:"buy_orders"`
		SellOrders []models.Order `json:"sell_orders"`
	}{
		BuyOrders:  buyOrders,
		SellOrders: sellOrders,
	}
	data, err := json.Marshal(orderBook)
	if err != nil {
		log.Printf("Failed to marshal order book: %v", err)
		return
	}

	clientsMu.RLock()
	for client := range clients {
		client.mu.Lock()
		err := client.conn.WriteMessage(websocket.TextMessage, data)
		client.mu.Unlock()
		if err != nil {
			log.Printf("Failed to send message: %v", err)
			clientsMu.RUnlock()
			clientsMu.Lock()
			delete(clients, client)
			clientsMu.Unlock()
			clientsMu.RLock()
		}
	}
	clientsMu.RUnlock()
}

func handleWebSocket(ex *exchange.Exchange) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("Failed to upgrade connection: %v", err)
			return
		}

		client := &WSClient{conn: conn}
		clientsMu.Lock()
		clients[client] = true
		clientsMu.Unlock()

		// Send initial order book
		broadcastOrderBook(ex)

		// Keep connection alive and handle disconnection
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				clientsMu.Lock()
				delete(clients, client)
				clientsMu.Unlock()
				break
			}
		}
	}
}

// Main entry point: sets up database, exchange, and HTTP server
func main() {
	ctx := context.Background()

	// Initialize database connection
	connString := "postgres://exchange_user:exchange_pass@localhost:5432/exchange_db?sslmode=disable"
	database, err := db.NewDB(ctx, connString)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close(ctx)

	// Initialize exchange (order book and matching engine)
	ex := exchange.NewExchange()

	// Initialize auth service
	authService := auth.NewAuthService(database)

	// Initialize API handlers
	handler := api.NewHandler(database, ex, authService)

	// Set up HTTP router
	r := chi.NewRouter()

	// Enable CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Serve static files
	r.Handle("/*", http.FileServer(http.Dir("frontend")))

	// WebSocket endpoint
	r.Get("/ws", handleWebSocket(ex))

	// Public endpoints
	r.Post("/auth/register", handler.Register)
	r.Post("/auth/login", handler.Login)

	// Protected endpoints (require JWT)
	r.Group(func(r chi.Router) {
		r.Use(handler.JWTAuthMiddleware)
		r.Post("/orders", handler.PlaceOrder)
		r.Get("/orders", handler.GetUserOrders)
		r.Delete("/orders/{id}", handler.CancelOrder)
		r.Get("/orderbook", handler.GetOrderBook)
		r.Get("/trades", handler.GetUserTrades)
	})

	// Start periodic order book broadcast
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			broadcastOrderBook(ex)
		}
	}()

	// Start server
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
