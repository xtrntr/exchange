package main

import (
	"context"
	"log"
	"net/http"
	"exchange/internal/api"
	"exchange/internal/auth"
	"exchange/internal/db"
	"exchange/internal/exchange"

	"github.com/go-chi/chi/v5"
)

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

	// Public endpoints
	r.Post("/auth/register", handler.Register)
	r.Post("/auth/login", handler.Login)

	// Protected endpoints (require JWT)
	r.Group(func(r chi.Router) {
		r.Use(handler.JWTAuthMiddleware)
		r.Post("/orders", handler.PlaceOrder)
		r.Get("/orders", handler.GetUserOrders)
		r.Get("/orderbook", handler.GetOrderBook)
		r.Get("/trades", handler.GetUserTrades)
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Start server
	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
} 