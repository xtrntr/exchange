package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/xtrntr/exchange/internal/auth"
	"github.com/xtrntr/exchange/internal/db"
	"github.com/xtrntr/exchange/internal/exchange"
	"github.com/xtrntr/exchange/internal/models"
)

// Handler contains dependencies for HTTP handlers
type Handler struct {
	DB          *db.DB
	Exchange    *exchange.Exchange
	AuthService *auth.AuthService
}

// NewHandler creates a new handler
func NewHandler(db *db.DB, ex *exchange.Exchange, authService *auth.AuthService) *Handler {
	return &Handler{DB: db, Exchange: ex, AuthService: authService}
}

// Register handles user registration
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, `{"error": "Username and password required"}`, http.StatusBadRequest)
		return
	}

	user, err := h.AuthService.Register(r.Context(), req.Username, req.Password)
	if err != nil {
		http.Error(w, `{"error": "Failed to register user"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":       user.ID,
		"username": user.Username,
	})
}

// Login handles user login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	token, err := h.AuthService.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		http.Error(w, `{"error": "Invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// JWTAuthMiddleware verifies JWT tokens
func (h *Handler) JWTAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Header.Get("Authorization")
		if tokenString == "" {
			http.Error(w, `{"error": "Authorization header required"}`, http.StatusUnauthorized)
			return
		}

		// Remove "Bearer " prefix if present
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		userID, err := h.AuthService.GetUserFromToken(tokenString)
		if err != nil {
			http.Error(w, `{"error": "Invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		// Add user_id to context
		ctx := context.WithValue(r.Context(), "user_id", userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// PlaceOrder handles order placement and matching
func (h *Handler) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		Type     string  `json:"type"`
		Price    float64 `json:"price"`
		Quantity float64 `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Type != "buy" && req.Type != "sell" {
		http.Error(w, `{"error": "Type must be 'buy' or 'sell'"}`, http.StatusBadRequest)
		return
	}
	if req.Price <= 0 || req.Quantity <= 0 {
		http.Error(w, `{"error": "Price and quantity must be positive"}`, http.StatusBadRequest)
		return
	}

	// Create order
	order := models.Order{
		UserID:   userID,
		Type:     req.Type,
		Price:    req.Price,
		Quantity: req.Quantity,
		Status:   "open",
	}

	// Save order to database
	dbOrder, err := h.DB.CreateOrder(r.Context(), &order)
	if err != nil {
		http.Error(w, `{"error": "Failed to create order"}`, http.StatusInternalServerError)
		return
	}

	// Try to match order
	trades, filledOrderIDs := h.Exchange.MatchOrder(*dbOrder)

	// Save trades to database
	for _, trade := range trades {
		_, err := h.DB.CreateTrade(r.Context(), &trade)
		if err != nil {
			http.Error(w, `{"error": "Failed to record trade"}`, http.StatusInternalServerError)
			return
		}
	}

	// Update filled orders
	for _, orderID := range filledOrderIDs {
		if err := h.DB.UpdateOrderStatus(r.Context(), orderID, "filled"); err != nil {
			http.Error(w, `{"error": "Failed to update order status"}`, http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Order placed",
		"order_id": dbOrder.ID,
	})
}

// GetUserOrders retrieves a user's orders
func (h *Handler) GetUserOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	orders, err := h.DB.GetUserOrders(r.Context(), userID)
	if err != nil {
		http.Error(w, `{"error": "Failed to retrieve orders"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(orders)
}

// GetOrderBook retrieves the current order book
func (h *Handler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	buyOrders, sellOrders := h.Exchange.GetOrderBook()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"buy_orders":  buyOrders,
		"sell_orders": sellOrders,
	})
}

// GetUserTrades retrieves a user's trade history
func (h *Handler) GetUserTrades(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	trades, err := h.DB.GetUserTrades(r.Context(), userID)
	if err != nil {
		http.Error(w, `{"error": "Failed to retrieve trades"}`, http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(trades)
}

// CancelOrder cancels an open order
func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	// Get order ID from URL
	orderIDStr := chi.URLParam(r, "id")
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		http.Error(w, `{"error": "Invalid order ID"}`, http.StatusBadRequest)
		return
	}

	// Cancel order in database
	err = h.DB.CancelOrder(r.Context(), orderID, userID)
	if err != nil {
		http.Error(w, `{"error": "Failed to cancel order: `+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	// Remove from order book
	if !h.Exchange.RemoveOrder(orderID) {
		// Log if order wasn't in book (non-fatal, as DB is source of truth)
		log.Printf("Order %d not found in order book", orderID)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Order canceled"})
} 