package exchange

import (
	"testing"
	"time"

	"github.com/xtrntr/exchange/internal/models"
)

func TestExchange_AddOrder(t *testing.T) {
	ex := NewExchange()

	// Test buy orders
	buyOrders := []models.Order{
		{
			ID:        1,
			Type:      "buy",
			Price:     50000,
			Quantity:  0.1,
			Status:    "open",
			CreatedAt: time.Now().Add(-time.Second),
		},
		{
			ID:        2,
			Type:      "buy",
			Price:     51000,
			Quantity:  0.2,
			Status:    "open",
			CreatedAt: time.Now(),
		},
		{
			ID:        3,
			Type:      "buy",
			Price:     50000,
			Quantity:  0.3,
			Status:    "open",
			CreatedAt: time.Now().Add(time.Second),
		},
	}

	for _, order := range buyOrders {
		ex.AddOrder(order)
	}

	if len(ex.BuyOrders) != 3 {
		t.Errorf("expected 3 buy orders, got %d", len(ex.BuyOrders))
	}

	// Verify price-time priority sorting
	if ex.BuyOrders[0].Price != 51000 {
		t.Errorf("expected highest price first, got %f", ex.BuyOrders[0].Price)
	}
	if ex.BuyOrders[1].Price == ex.BuyOrders[2].Price && ex.BuyOrders[1].CreatedAt.After(ex.BuyOrders[2].CreatedAt) {
		t.Error("buy orders with same price not sorted by time")
	}

	// Test sell orders
	sellOrders := []models.Order{
		{
			ID:        4,
			Type:      "sell",
			Price:     52000,
			Quantity:  0.1,
			Status:    "open",
			CreatedAt: time.Now().Add(-time.Second),
		},
		{
			ID:        5,
			Type:      "sell",
			Price:     51000,
			Quantity:  0.2,
			Status:    "open",
			CreatedAt: time.Now(),
		},
		{
			ID:        6,
			Type:      "sell",
			Price:     52000,
			Quantity:  0.3,
			Status:    "open",
			CreatedAt: time.Now().Add(time.Second),
		},
	}

	for _, order := range sellOrders {
		ex.AddOrder(order)
	}

	if len(ex.SellOrders) != 3 {
		t.Errorf("expected 3 sell orders, got %d", len(ex.SellOrders))
	}

	// Verify price-time priority sorting
	if ex.SellOrders[0].Price != 51000 {
		t.Errorf("expected lowest price first, got %f", ex.SellOrders[0].Price)
	}
	if ex.SellOrders[1].Price == ex.SellOrders[2].Price && ex.SellOrders[1].CreatedAt.After(ex.SellOrders[2].CreatedAt) {
		t.Error("sell orders with same price not sorted by time")
	}
}

func TestExchange_MatchOrder(t *testing.T) {
	ex := NewExchange()

	// Pre-populate order book
	sellOrders := []models.Order{
		{
			ID:        1,
			Type:      "sell",
			Price:     50000,
			Quantity:  0.1,
			Status:    "open",
			CreatedAt: time.Now().Add(-time.Second),
		},
		{
			ID:        2,
			Type:      "sell",
			Price:     50000,
			Quantity:  0.05,
			Status:    "open",
			CreatedAt: time.Now(),
		},
		{
			ID:        3,
			Type:      "sell",
			Price:     51000,
			Quantity:  0.2,
			Status:    "open",
			CreatedAt: time.Now(),
		},
	}

	for _, order := range sellOrders {
		ex.AddOrder(order)
	}

	tests := []struct {
		name         string
		order        models.Order
		expectTrades int
		expectFilled []int
	}{
		{
			name: "MatchWithTimePriority",
			order: models.Order{
				ID:       4,
				Type:     "buy",
				Price:    51000,
				Quantity: 0.1,
				Status:   "open",
			},
			expectTrades: 1,
			expectFilled: []int{1, 4},
		},
		{
			name: "PartialFill",
			order: models.Order{
				ID:       5,
				Type:     "buy",
				Price:    50000,
				Quantity: 0.02,
				Status:   "open",
			},
			expectTrades: 1,
			expectFilled: []int{5},
		},
		{
			name: "NoMatch",
			order: models.Order{
				ID:       6,
				Type:     "buy",
				Price:    49000,
				Quantity: 0.1,
				Status:   "open",
			},
			expectTrades: 0,
			expectFilled: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trades, filled := ex.MatchOrder(tt.order)

			if len(trades) != tt.expectTrades {
				t.Errorf("expected %d trades, got %d", tt.expectTrades, len(trades))
			}

			if len(filled) != len(tt.expectFilled) {
				t.Errorf("expected %d filled orders, got %d", len(tt.expectFilled), len(filled))
			}

			for _, id := range tt.expectFilled {
				found := false
				for _, fid := range filled {
					if fid == id {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected order %d to be filled", id)
				}
			}
		})
	}
}

func TestExchange_RemoveOrder(t *testing.T) {
	ex := NewExchange()

	// Add test orders
	orders := []models.Order{
		{
			ID:       1,
			Type:     "buy",
			Price:    50000,
			Quantity: 0.1,
			Status:   "open",
		},
		{
			ID:       2,
			Type:     "sell",
			Price:    51000,
			Quantity: 0.2,
			Status:   "open",
		},
	}

	for _, order := range orders {
		ex.AddOrder(order)
	}

	tests := []struct {
		name          string
		orderID       int
		expectRemoved bool
	}{
		{
			name:          "RemoveBuyOrder",
			orderID:       1,
			expectRemoved: true,
		},
		{
			name:          "RemoveSellOrder",
			orderID:       2,
			expectRemoved: true,
		},
		{
			name:          "NonExistentOrder",
			orderID:       999,
			expectRemoved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			removed := ex.RemoveOrder(tt.orderID)
			if removed != tt.expectRemoved {
				t.Errorf("expected removed=%v, got %v", tt.expectRemoved, removed)
			}

			// Verify order is not in either book
			for _, o := range ex.BuyOrders {
				if o.ID == tt.orderID {
					t.Errorf("order %d still in buy orders", tt.orderID)
				}
			}
			for _, o := range ex.SellOrders {
				if o.ID == tt.orderID {
					t.Errorf("order %d still in sell orders", tt.orderID)
				}
			}
		})
	}
}

func TestExchange_GetOrderBook(t *testing.T) {
	ex := NewExchange()

	// Add test orders
	orders := []models.Order{
		{
			ID:        1,
			Type:      "buy",
			Price:     50000,
			Quantity:  0.1,
			Status:    "open",
			CreatedAt: time.Now().Add(-time.Second),
		},
		{
			ID:        2,
			Type:      "sell",
			Price:     51000,
			Quantity:  0.2,
			Status:    "open",
			CreatedAt: time.Now(),
		},
		{
			ID:        3,
			Type:      "buy",
			Price:     49000,
			Quantity:  0.3,
			Status:    "open",
			CreatedAt: time.Now().Add(time.Second),
		},
	}

	for _, order := range orders {
		ex.AddOrder(order)
	}

	buyOrders, sellOrders := ex.GetOrderBook()

	if len(buyOrders) != 2 {
		t.Errorf("expected 2 buy orders, got %d", len(buyOrders))
	}
	if len(sellOrders) != 1 {
		t.Errorf("expected 1 sell order, got %d", len(sellOrders))
	}

	// Verify buy orders are sorted by price-time priority
	if len(buyOrders) >= 2 && buyOrders[0].Price < buyOrders[1].Price {
		t.Error("buy orders not sorted by price (highest first)")
	}

	// Verify sell orders are sorted by price-time priority
	if len(sellOrders) >= 2 && sellOrders[0].Price > sellOrders[1].Price {
		t.Error("sell orders not sorted by price (lowest first)")
	}
}
