package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/xtrntr/exchange/internal/auth"
	"github.com/xtrntr/exchange/internal/db"
	"github.com/xtrntr/exchange/internal/exchange"
	"github.com/xtrntr/exchange/internal/models"
)

var (
	testDB      *db.DB
	testAuth    *auth.AuthService
	testEx      *exchange.Exchange
	testRouter  *chi.Mux
	testPool    *pgxpool.Pool
	testHandler *Handler
)

const testDBConnString = "postgres://exchange_user:exchange_pass@localhost:5432/exchange_db?sslmode=disable"

func TestMain(m *testing.M) {
	var err error
	ctx := context.Background()

	// Connect to test database
	testPool, err = pgxpool.New(ctx, testDBConnString)
	if err != nil {
		fmt.Printf("Failed to connect to test database: %v\n", err)
		os.Exit(1)
	}
	defer testPool.Close()

	// Initialize test dependencies
	testDB, err = db.NewDB(ctx, testDBConnString)
	if err != nil {
		fmt.Printf("Failed to create DB: %v\n", err)
		os.Exit(1)
	}
	testAuth = auth.NewAuthService(testDB)
	testEx = exchange.NewExchange()

	// Create handler and router
	testHandler = NewHandler(testDB, testEx, testAuth)
	testRouter = chi.NewRouter()
	testRouter.Post("/register", testHandler.Register)
	testRouter.Post("/login", testHandler.Login)

	// Protected routes
	testRouter.Group(func(r chi.Router) {
		r.Use(testHandler.JWTAuthMiddleware)
		r.Post("/orders", testHandler.PlaceOrder)
		r.Delete("/orders/{id}", testHandler.CancelOrder)
		r.Get("/orders", testHandler.GetUserOrders)
		r.Get("/orderbook", testHandler.GetOrderBook)
		r.Get("/trades", testHandler.GetUserTrades)
	})

	// Run tests
	code := m.Run()

	// Clean up
	os.Exit(code)
}

func cleanupDB(t *testing.T) {
	ctx := context.Background()
	_, err := testPool.Exec(ctx, "TRUNCATE users, orders, trades RESTART IDENTITY")
	assert.NoError(t, err)
	testEx = exchange.NewExchange()                    // Reset exchange state
	testHandler = NewHandler(testDB, testEx, testAuth) // Update handler with new exchange

	// Update router with new handler
	testRouter = chi.NewRouter()
	testRouter.Post("/register", testHandler.Register)
	testRouter.Post("/login", testHandler.Login)

	// Protected routes
	testRouter.Group(func(r chi.Router) {
		r.Use(testHandler.JWTAuthMiddleware)
		r.Post("/orders", testHandler.PlaceOrder)
		r.Delete("/orders/{id}", testHandler.CancelOrder)
		r.Get("/orders", testHandler.GetUserOrders)
		r.Get("/orderbook", testHandler.GetOrderBook)
		r.Get("/trades", testHandler.GetUserTrades)
	})
}

func TestHandler_Register(t *testing.T) {
	cleanupDB(t)

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "Success",
			requestBody: map[string]interface{}{
				"username": "testuser",
				"password": "testpass",
			},
			expectedStatus: http.StatusCreated,
			expectedBody: map[string]interface{}{
				"id":       float64(1), // JSON numbers are float64
				"username": "testuser",
			},
		},
		{
			name: "Missing Password",
			requestBody: map[string]interface{}{
				"username": "testuser",
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"error": "Username and password required",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/register", bytes.NewReader(body))
			w := httptest.NewRecorder()

			testRouter.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBody, response)
		})
	}
}

func TestHandler_Login(t *testing.T) {
	cleanupDB(t)

	// Create a test user
	ctx := context.Background()
	_, err := testAuth.Register(ctx, "testuser", "testpass")
	assert.NoError(t, err)

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		expectToken    bool
	}{
		{
			name: "Success",
			requestBody: map[string]interface{}{
				"username": "testuser",
				"password": "testpass",
			},
			expectedStatus: http.StatusOK,
			expectToken:    true,
		},
		{
			name: "Invalid Credentials",
			requestBody: map[string]interface{}{
				"username": "testuser",
				"password": "wrongpass",
			},
			expectedStatus: http.StatusUnauthorized,
			expectToken:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
			w := httptest.NewRecorder()

			testRouter.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tt.expectToken {
				assert.Contains(t, response, "token")
				assert.NotEmpty(t, response["token"])
			} else {
				assert.Contains(t, response, "error")
			}
		})
	}
}

func TestHandler_PlaceOrder(t *testing.T) {
	cleanupDB(t)

	// Create a test user and get token
	ctx := context.Background()
	_, err := testAuth.Register(ctx, "testuser", "testpass")
	assert.NoError(t, err)

	token, err := testAuth.Login(ctx, "testuser", "testpass")
	assert.NoError(t, err)

	tests := []struct {
		name           string
		requestBody    map[string]interface{}
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "Success - Buy Order",
			requestBody: map[string]interface{}{
				"type":     "buy",
				"price":    100.0,
				"quantity": 1.0,
			},
			expectedStatus: http.StatusCreated,
			expectedBody: map[string]interface{}{
				"message":  "Order placed",
				"order_id": float64(1),
			},
		},
		{
			name: "Invalid Order Type",
			requestBody: map[string]interface{}{
				"type":     "invalid",
				"price":    100.0,
				"quantity": 1.0,
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: map[string]interface{}{
				"error": "Type must be 'buy' or 'sell'",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest("POST", "/orders", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()

			testRouter.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedBody, response)
		})
	}
}

func TestHandler_GetOrderBook(t *testing.T) {
	cleanupDB(t)

	// Create a test user and get token
	ctx := context.Background()
	_, err := testAuth.Register(ctx, "testuser", "testpass")
	assert.NoError(t, err)

	token, err := testAuth.Login(ctx, "testuser", "testpass")
	assert.NoError(t, err)

	// Place some test orders
	orders := []models.Order{
		{
			UserID:   1,
			Type:     "buy",
			Price:    100.0,
			Quantity: 1.0,
			Status:   "open",
		},
		{
			UserID:   1,
			Type:     "sell",
			Price:    110.0,
			Quantity: 1.0,
			Status:   "open",
		},
	}

	for _, order := range orders {
		dbOrder, err := testDB.CreateOrder(ctx, &order)
		assert.NoError(t, err)
		testEx.AddOrder(*dbOrder)
	}

	req := httptest.NewRequest("GET", "/orderbook", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	testRouter.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	buyOrders, ok := response["buy_orders"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, buyOrders, 1)

	sellOrders, ok := response["sell_orders"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, sellOrders, 1)
}

func TestHandler_CancelOrder(t *testing.T) {
	cleanupDB(t)

	// Create a test user and get token
	ctx := context.Background()
	_, err := testAuth.Register(ctx, "testuser", "testpass")
	assert.NoError(t, err)

	token, err := testAuth.Login(ctx, "testuser", "testpass")
	assert.NoError(t, err)

	// Place a test order
	order := models.Order{
		UserID:   1,
		Type:     "buy",
		Price:    100.0,
		Quantity: 1.0,
		Status:   "open",
	}
	dbOrder, err := testDB.CreateOrder(ctx, &order)
	assert.NoError(t, err)
	testEx.AddOrder(*dbOrder)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/orders/%d", dbOrder.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	testRouter.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Order canceled", response["message"])
}
