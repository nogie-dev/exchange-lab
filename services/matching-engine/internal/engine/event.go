package engine

import "github.com/nogie-dev/clob-trading/internal/models"

type EventType int

const (
	NewOrder EventType = iota
	CancelOrder
	EditOrder
	snapshotOrderBook
)

type Event struct {
	Type   EventType
	Ticker string

	NewOrder      *models.CreateOrderRequest
	CancelReq     *models.CancelOrderRequest
	EditReq       *models.EditOrderRequest
	commandResult chan<- error

	snapshotDepth  int
	snapshotResult chan<- OrderBookSnapshot
}

func (e Event) acknowledge(err error) {
	if e.commandResult != nil {
		e.commandResult <- err
	}
}
