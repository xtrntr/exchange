package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
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

func broadcastOrderBook(ex *exchange.Exchange, database *db.DB) {
	// Get open orders directly from database
	ctx := context.Background()
	openOrders, err := database.GetOpenOrders(ctx)
	if err != nil {
		log.Printf("Failed to get open orders from database: %v", err)
		return
	}

	// Separate into buy and sell orders
	var buyOrders, sellOrders []models.Order
	for _, order := range openOrders {
		if order.Type == "buy" {
			buyOrders = append(buyOrders, order)
		} else {
			sellOrders = append(sellOrders, order)
		}
	}

	// Sort orders appropriately
	sort.Slice(buyOrders, func(i, j int) bool {
		if buyOrders[i].Price == buyOrders[j].Price {
			return buyOrders[i].CreatedAt.Before(buyOrders[j].CreatedAt)
		}
		return buyOrders[i].Price > buyOrders[j].Price
	})

	sort.Slice(sellOrders, func(i, j int) bool {
		if sellOrders[i].Price == sellOrders[j].Price {
			return sellOrders[i].CreatedAt.Before(sellOrders[j].CreatedAt)
		}
		return sellOrders[i].Price < sellOrders[j].Price
	})

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

func handleWebSocket(ex *exchange.Exchange, database *db.DB) http.HandlerFunc {
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

		// Send initial order book from database
		broadcastOrderBook(ex, database)

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

	// Load open orders into exchange
	openOrders, err := database.GetOpenOrders(ctx)
	if err != nil {
		log.Printf("Failed to load open orders: %v", err)
	} else {
		for _, order := range openOrders {
			ex.AddOrder(order)
		}
		log.Printf("Loaded %d open orders into exchange", len(openOrders))
	}

	// Initialize auth service
	authService := auth.NewAuthService(database)

	// Initialize API handlers
	handler := api.NewHandler(database, ex, authService)

	// Set up HTTP router
	r := chi.NewRouter()

	// Enable CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"}, // Only allow the React dev server
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// WebSocket endpoint
	r.Get("/ws", handleWebSocket(ex, database))

	// Public endpoints
	r.Post("/register", handler.Register)
	r.Post("/login", handler.Login)

	// Protected endpoints (require JWT)
	r.Group(func(r chi.Router) {
		r.Use(handler.JWTAuthMiddleware)
		r.Post("/orders", handler.PlaceOrder)
		r.Get("/orders", handler.GetUserOrders)
		r.Delete("/orders/{id}", handler.CancelOrder)
		r.Get("/orderbook", handler.GetOrderBook)
		r.Get("/trades", handler.GetUserTrades)
		r.Get("/trades/all", handler.GetAllTrades)
		r.Get("/debug/auth", func(w http.ResponseWriter, r *http.Request) {
			userID, ok := r.Context().Value("user_id").(int)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			response := fmt.Sprintf(`{"status":"success","user_id":%d,"authenticated":true}`, userID)
			w.Write([]byte(response))
		})
	})

	// Start periodic order book broadcast using database as source of truth
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			broadcastOrderBook(ex, database)
		}
	}()

	// Start server
	log.Printf("Starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
