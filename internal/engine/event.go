package engine

import "github.com/nogie-dev/clob-trading/internal/models"

type EventType int

const (
	NewOrder EventType = iota
	CancelOrder
	EditOrder
)

type Event struct {
	Type   EventType
	Ticker string

	NewOrder  *models.CreateOrderRequest
	CancelReq *models.CancelOrderRequest
	EditReq   *models.EditOrderRequest
}
