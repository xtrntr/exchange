package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sort"
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

// writeJSON writes a JSON response with consistent formatting
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		response, _ := json.Marshal(data)
		w.Write(response)
	}
}

// writeError writes a JSON error response with consistent formatting
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// Register handles user registration
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "Username and password required")
		return
	}

	user, err := h.AuthService.Register(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to register user")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
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
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	token, err := h.AuthService.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// JWTAuthMiddleware verifies JWT tokens
func (h *Handler) JWTAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Header.Get("Authorization")
		if tokenString == "" {
			writeError(w, http.StatusUnauthorized, "Authorization header required")
			return
		}

		// Remove "Bearer " prefix if present
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		userID, err := h.AuthService.GetUserFromToken(tokenString)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Invalid or expired token")
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
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Type     string  `json:"type"`
		Price    float64 `json:"price"`
		Quantity float64 `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	if req.Type != "buy" && req.Type != "sell" {
		writeError(w, http.StatusBadRequest, "Type must be 'buy' or 'sell'")
		return
	}
	if req.Price <= 0 || req.Quantity <= 0 {
		writeError(w, http.StatusBadRequest, "Price and quantity must be positive")
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
		writeError(w, http.StatusInternalServerError, "Failed to create order")
		return
	}

	// Try to match order
	trades, filledOrderIDs := h.Exchange.MatchOrder(*dbOrder)

	// Save trades to database
	for _, trade := range trades {
		_, err := h.DB.CreateTrade(r.Context(), &trade)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to record trade")
			return
		}
	}

	// Update filled orders
	for _, orderID := range filledOrderIDs {
		if err := h.DB.UpdateOrderStatus(r.Context(), orderID, "filled"); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update order status")
			return
		}
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":  "Order placed",
		"order_id": dbOrder.ID,
	})
}

// GetUserOrders retrieves a user's orders
func (h *Handler) GetUserOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	orders, err := h.DB.GetUserOrders(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve orders")
		return
	}

	writeJSON(w, http.StatusOK, orders)
}

// GetOrderBook retrieves the current order book
func (h *Handler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	// Get open orders directly from database
	orders, err := h.DB.GetOpenOrders(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve order book")
		return
	}

	// Separate into buy and sell orders
	var buyOrders, sellOrders []models.Order
	for _, order := range orders {
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"buy_orders":  buyOrders,
		"sell_orders": sellOrders,
	})
}

// GetUserTrades retrieves a user's trade history
func (h *Handler) GetUserTrades(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	trades, err := h.DB.GetUserTrades(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve trades")
		return
	}

	writeJSON(w, http.StatusOK, trades)
}

// CancelOrder cancels an open order
func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value("user_id").(int)
	if !ok {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Get order ID from URL
	orderIDStr := chi.URLParam(r, "id")
	orderID, err := strconv.Atoi(orderIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	// Cancel order in database
	err = h.DB.CancelOrder(r.Context(), orderID, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to cancel order: "+err.Error())
		return
	}

	// Remove from order book
	if !h.Exchange.RemoveOrder(orderID) {
		// Log if order wasn't in book (non-fatal, as DB is source of truth)
		log.Printf("Order %d not found in order book", orderID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Order canceled"})
}
