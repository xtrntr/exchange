package exchange

import (
	"sort"
	"exchange/internal/models"
)

// Exchange manages the order book and matching engine
type Exchange struct {
	BuyOrders  []models.Order
	SellOrders []models.Order
}

// NewExchange creates a new exchange
func NewExchange() *Exchange {
	return &Exchange{
		BuyOrders:  []models.Order{},
		SellOrders: []models.Order{},
	}
}

// AddOrder adds an order to the order book
func (e *Exchange) AddOrder(order models.Order) {
	if order.Type == "buy" {
		e.BuyOrders = append(e.BuyOrders, order)
		// Sort buy orders: highest price first, then earliest time
		sort.Slice(e.BuyOrders, func(i, j int) bool {
			if e.BuyOrders[i].Price == e.BuyOrders[j].Price {
				return e.BuyOrders[i].CreatedAt.Before(e.BuyOrders[j].CreatedAt)
			}
			return e.BuyOrders[i].Price > e.BuyOrders[j].Price
		})
	} else {
		e.SellOrders = append(e.SellOrders, order)
		// Sort sell orders: lowest price first, then earliest time
		sort.Slice(e.SellOrders, func(i, j int) bool {
			if e.SellOrders[i].Price == e.SellOrders[j].Price {
				return e.SellOrders[i].CreatedAt.Before(e.SellOrders[j].CreatedAt)
			}
			return e.SellOrders[i].Price < e.SellOrders[j].Price
		})
	}
}

// MatchOrder attempts to match a new order, returns trades
func (e *Exchange) MatchOrder(newOrder models.Order) ([]models.Trade, []int) {
	var trades []models.Trade
	var filledOrderIDs []int

	if newOrder.Type == "buy" {
		// Match against sell orders
		for i := 0; i < len(e.SellOrders); i++ {
			if e.SellOrders[i].Status != "open" || newOrder.Quantity <= 0 {
				continue
			}
			if e.SellOrders[i].Price <= newOrder.Price {
				// Calculate trade quantity
				tradeQty := min(newOrder.Quantity, e.SellOrders[i].Quantity)
				tradePrice := e.SellOrders[i].Price // Use sell price for simplicity

				// Create trade
				trade := models.Trade{
					BuyOrderID:  newOrder.ID,
					SellOrderID: e.SellOrders[i].ID,
					Price:       tradePrice,
					Quantity:    tradeQty,
				}
				trades = append(trades, trade)

				// Update quantities
				newOrder.Quantity -= tradeQty
				e.SellOrders[i].Quantity -= tradeQty

				// Mark orders as filled if quantity is 0
				if newOrder.Quantity <= 0 {
					filledOrderIDs = append(filledOrderIDs, newOrder.ID)
				}
				if e.SellOrders[i].Quantity <= 0 {
					filledOrderIDs = append(filledOrderIDs, e.SellOrders[i].ID)
					e.SellOrders[i].Status = "filled"
				}
			}
			if newOrder.Quantity <= 0 {
				break
			}
		}
	} else {
		// Match against buy orders
		for i := 0; i < len(e.BuyOrders); i++ {
			if e.BuyOrders[i].Status != "open" || newOrder.Quantity <= 0 {
				continue
			}
			if e.BuyOrders[i].Price >= newOrder.Price {
				tradeQty := min(newOrder.Quantity, e.BuyOrders[i].Quantity)
				tradePrice := e.BuyOrders[i].Price // Use buy price for simplicity

				trade := models.Trade{
					BuyOrderID:  e.BuyOrders[i].ID,
					SellOrderID: newOrder.ID,
					Price:       tradePrice,
					Quantity:    tradeQty,
				}
				trades = append(trades, trade)

				newOrder.Quantity -= tradeQty
				e.BuyOrders[i].Quantity -= tradeQty

				if newOrder.Quantity <= 0 {
					filledOrderIDs = append(filledOrderIDs, newOrder.ID)
				}
				if e.BuyOrders[i].Quantity <= 0 {
					filledOrderIDs = append(filledOrderIDs, e.BuyOrders[i].ID)
					e.BuyOrders[i].Status = "filled"
				}
			}
			if newOrder.Quantity <= 0 {
				break
			}
		}
	}

	// Update order book: remove filled orders
	e.cleanupOrderBook()

	// Add remaining new order to book if not fully filled
	if newOrder.Quantity > 0 && newOrder.Status == "open" {
		e.AddOrder(newOrder)
	}

	return trades, filledOrderIDs
}

// cleanupOrderBook removes filled orders
func (e *Exchange) cleanupOrderBook() {
	var newBuyOrders []models.Order
	for _, order := range e.BuyOrders {
		if order.Status == "open" && order.Quantity > 0 {
			newBuyOrders = append(newBuyOrders, order)
		}
	}
	e.BuyOrders = newBuyOrders

	var newSellOrders []models.Order
	for _, order := range e.SellOrders {
		if order.Status == "open" && order.Quantity > 0 {
			newSellOrders = append(newSellOrders, order)
		}
	}
	e.SellOrders = newSellOrders
}

// GetOrderBook returns the current order book
func (e *Exchange) GetOrderBook() ([]models.Order, []models.Order) {
	return e.BuyOrders, e.SellOrders
}

// min returns the smaller of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
} 