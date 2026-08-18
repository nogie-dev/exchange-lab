package orderevent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/nogie-dev/clob-trading/internal/models"
)

var (
	ErrInvalidEvent     = errors.New("invalid order event")
	ErrStoreUnavailable = errors.New("order event store unavailable")
)

type Type string

const (
	Resting           Type = "ORDER_RESTING"
	PartiallyFilled   Type = "ORDER_PARTIALLY_FILLED"
	Filled            Type = "ORDER_FILLED"
	Amended           Type = "ORDER_AMENDED"
	Canceled          Type = "ORDER_CANCELED"
	RemainderCanceled Type = "ORDER_REMAINDER_CANCELED"
)

const (
	ReasonNone                  = "NONE"
	ReasonNoMatch               = "NO_MATCH"
	ReasonPartialFill           = "PARTIAL_FILL"
	ReasonPriceChanged          = "PRICE_CHANGED"
	ReasonAmountIncreased       = "AMOUNT_INCREASED"
	ReasonAmountDecreased       = "AMOUNT_DECREASED"
	ReasonUserRequest           = "USER_REQUEST"
	ReasonInsufficientLiquidity = "INSUFFICIENT_LIQUIDITY"
)

// Event is one durable state transition for an order. Match logs remain the
// source for individual executions; order events describe the resulting order
// lifecycle for both maker and taker orders.
type Event struct {
	EventID         string
	CommandID       string
	CommandSequence int64
	EventIndex      int32
	OrderID         string
	UserID          string
	Ticker          string
	Type            Type
	Reason          string
	OrderType       models.OrderType
	Side            models.Position
	PreviousPrice   float64
	Price           float64
	PreviousAmount  float64
	FilledAmount    float64
	CanceledAmount  float64
	RemainingAmount float64
	OccurredAt      time.Time
}

type Store interface {
	SaveEvent(ctx context.Context, event Event) error
	SaveEvents(ctx context.Context, events []Event) error
}

// GenerateEventID returns a stable identity for one order transition produced
// by a journaled command. EventIndex is the deterministic transition order
// within that command.
func GenerateEventID(ticker, commandID, orderID string, eventType Type, eventIndex int32) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%d", ticker, commandID, orderID, eventType, eventIndex)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func Validate(event Event) error {
	switch {
	case strings.TrimSpace(event.EventID) == "":
		return fmt.Errorf("%w: event id is required", ErrInvalidEvent)
	case strings.TrimSpace(event.CommandID) == "":
		return fmt.Errorf("%w: command id is required", ErrInvalidEvent)
	case event.CommandSequence <= 0:
		return fmt.Errorf("%w: command sequence must be positive", ErrInvalidEvent)
	case event.EventIndex < 0:
		return fmt.Errorf("%w: event index cannot be negative", ErrInvalidEvent)
	case strings.TrimSpace(event.OrderID) == "":
		return fmt.Errorf("%w: order id is required", ErrInvalidEvent)
	case strings.TrimSpace(event.UserID) == "":
		return fmt.Errorf("%w: user id is required", ErrInvalidEvent)
	case strings.TrimSpace(event.Ticker) == "":
		return fmt.Errorf("%w: ticker is required", ErrInvalidEvent)
	case !validType(event.Type):
		return fmt.Errorf("%w: unsupported type %q", ErrInvalidEvent, event.Type)
	case strings.TrimSpace(event.Reason) == "":
		return fmt.Errorf("%w: reason is required", ErrInvalidEvent)
	case event.OrderType != models.Limit && event.OrderType != models.Market:
		return fmt.Errorf("%w: unsupported order type %q", ErrInvalidEvent, event.OrderType)
	case event.Side != models.Bid && event.Side != models.Ask:
		return fmt.Errorf("%w: side must be BID or ASK", ErrInvalidEvent)
	case !validPrices(event):
		return fmt.Errorf("%w: invalid order prices", ErrInvalidEvent)
	case !positiveFinite(event.PreviousAmount):
		return fmt.Errorf("%w: previous amount must be positive", ErrInvalidEvent)
	case !nonNegativeFinite(event.FilledAmount) || !nonNegativeFinite(event.CanceledAmount) || !nonNegativeFinite(event.RemainingAmount):
		return fmt.Errorf("%w: event amounts cannot be negative or non-finite", ErrInvalidEvent)
	case event.OccurredAt.IsZero():
		return fmt.Errorf("%w: occurred time is required", ErrInvalidEvent)
	}
	expectedID := GenerateEventID(event.Ticker, event.CommandID, event.OrderID, event.Type, event.EventIndex)
	if event.EventID != expectedID {
		return fmt.Errorf("%w: event id does not match transition identity", ErrInvalidEvent)
	}

	if err := validateTransition(event); err != nil {
		return err
	}
	return nil
}

func validType(eventType Type) bool {
	switch eventType {
	case Resting, PartiallyFilled, Filled, Amended, Canceled, RemainderCanceled:
		return true
	default:
		return false
	}
}

func validPrices(event Event) bool {
	if !nonNegativeFinite(event.PreviousPrice) || !nonNegativeFinite(event.Price) {
		return false
	}
	if event.OrderType == models.Market {
		return event.PreviousPrice == 0 && event.Price == 0
	}
	return event.PreviousPrice > 0 && event.Price > 0
}

func validateTransition(event Event) error {
	switch event.Type {
	case Resting:
		if event.FilledAmount != 0 || event.CanceledAmount != 0 || event.RemainingAmount <= 0 || !sameAmount(event.PreviousAmount, event.RemainingAmount) {
			return fmt.Errorf("%w: invalid resting transition", ErrInvalidEvent)
		}
	case PartiallyFilled:
		if event.FilledAmount <= 0 || event.CanceledAmount != 0 || event.RemainingAmount <= 0 || !sameAmount(event.PreviousAmount, event.FilledAmount+event.RemainingAmount) {
			return fmt.Errorf("%w: invalid partial-fill transition", ErrInvalidEvent)
		}
	case Filled:
		if event.FilledAmount <= 0 || event.CanceledAmount != 0 || event.RemainingAmount != 0 || !sameAmount(event.PreviousAmount, event.FilledAmount) {
			return fmt.Errorf("%w: invalid filled transition", ErrInvalidEvent)
		}
	case Amended:
		if event.FilledAmount != 0 || event.CanceledAmount != 0 || event.RemainingAmount <= 0 {
			return fmt.Errorf("%w: invalid amended transition", ErrInvalidEvent)
		}
	case Canceled, RemainderCanceled:
		if event.FilledAmount != 0 || event.CanceledAmount <= 0 || event.RemainingAmount != 0 || !sameAmount(event.PreviousAmount, event.CanceledAmount) {
			return fmt.Errorf("%w: invalid canceled transition", ErrInvalidEvent)
		}
	}
	return nil
}

func positiveFinite(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func nonNegativeFinite(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func sameAmount(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}
