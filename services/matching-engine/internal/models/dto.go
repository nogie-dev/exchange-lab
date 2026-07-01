package models

// CancelOrderRequest carries the minimal info to cancel an order.
type CancelOrderRequest struct {
	Ticker  string `json:"ticker"`
	OrderID string `json:"order_id"`
}

// EditOrderRequest describes an order modification.
// Amount is optional; nil means no change.
type EditOrderRequest struct {
	Ticker  string   `json:"ticker"`
	OrderID string   `json:"order_id"`
	Price   float64  `json:"price"`
	Amount  *float64 `json:"amount"`
}

// CreateOrderRequest is the incoming order DTO for new orders.
type CreateOrderRequest struct {
	Ticker    string    `json:"ticker"`
	UserID    string    `json:"user_id"`
	OrderType OrderType `json:"order_type"`
	Position  Position  `json:"position"`
	Price     float64   `json:"price"`
	Amount    float64   `json:"amount"`
	Nonce     uint64    `json:"nonce"`
}
