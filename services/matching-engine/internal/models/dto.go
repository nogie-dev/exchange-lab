package models

import "github.com/nogie-dev/clob-trading/internal/numeric"

// CancelOrderRequest carries the minimal info to cancel an order.
type CancelOrderRequest struct {
	CommandID string `json:"command_id"`
	Ticker    string `json:"ticker"`
	OrderID   string `json:"order_id"`
}

// EditOrderRequest describes an order modification.
// Amount is optional; nil means no change.
type EditOrderRequest struct {
	CommandID string                `json:"command_id"`
	Ticker    string                `json:"ticker"`
	OrderID   string                `json:"order_id"`
	Price     numeric.PriceTicks    `json:"price"`
	Amount    *numeric.QuantityLots `json:"amount"`
}

// CreateOrderRequest is the canonical journal/engine command for a new order.
// HTTP decimal strings are converted to these fixed-point units in internal/api.
type CreateOrderRequest struct {
	CommandID string               `json:"command_id"`
	Ticker    string               `json:"ticker"`
	UserID    string               `json:"user_id"`
	OrderType OrderType            `json:"order_type"`
	Position  Position             `json:"position"`
	Price     numeric.PriceTicks   `json:"price"`
	Amount    numeric.QuantityLots `json:"amount"`
	Nonce     uint64               `json:"nonce"`
}
