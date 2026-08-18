package journal

import (
	"errors"
	"testing"
	"time"

	"github.com/nogie-dev/clob-trading/internal/models"
)

func TestCommandPayloadRoundTrip(t *testing.T) {
	command := Command{
		CommandID: "command-1",
		Ticker:    "BTC-USD",
		Type:      CreateCommand,
		Create: &models.CreateOrderRequest{
			CommandID: "command-1",
			Ticker:    "BTC-USD",
			UserID:    "alice",
			OrderType: models.Limit,
			Position:  models.Bid,
			Price:     100,
			Amount:    1,
			Nonce:     1,
		},
	}
	payload, err := EncodePayload(command)
	if err != nil {
		t.Fatalf("EncodePayload returned error: %v", err)
	}
	recordedAt := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	decoded, err := Decode(command.CommandID, command.Ticker, 7, command.Type, payload, recordedAt)
	if err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if !SamePayload(command, decoded) || decoded.Sequence != 7 || decoded.RecordedAt != recordedAt {
		t.Fatalf("unexpected decoded command: %#v", decoded)
	}
}

func TestValidateRejectsPayloadIdentityMismatch(t *testing.T) {
	command := Command{
		CommandID: "command-1",
		Ticker:    "BTC-USD",
		Type:      CancelCommand,
		Cancel: &models.CancelOrderRequest{
			CommandID: "command-2",
			Ticker:    "BTC-USD",
			OrderID:   "order-1",
		},
	}
	if err := Validate(command); !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("Validate want ErrInvalidCommand, got %v", err)
	}
}

func TestValidateAcceptsMarketCreateWithoutPrice(t *testing.T) {
	command := Command{
		CommandID: "market-command",
		Ticker:    "BTC-USD",
		Type:      CreateCommand,
		Create: &models.CreateOrderRequest{
			CommandID: "market-command",
			Ticker:    "BTC-USD",
			UserID:    "alice",
			OrderType: models.Market,
			Position:  models.Bid,
			Amount:    1,
		},
	}
	if err := Validate(command); err != nil {
		t.Fatalf("Validate returned error for market order: %v", err)
	}
}

func TestMarketCommandPayloadRoundTrip(t *testing.T) {
	command := Command{
		CommandID: "market-command",
		Ticker:    "BTC-USD",
		Type:      CreateCommand,
		Create: &models.CreateOrderRequest{
			CommandID: "market-command",
			Ticker:    "BTC-USD",
			UserID:    "alice",
			OrderType: models.Market,
			Position:  models.Ask,
			Amount:    1,
			Nonce:     2,
		},
	}
	payload, err := EncodePayload(command)
	if err != nil {
		t.Fatalf("EncodePayload returned error: %v", err)
	}
	recordedAt := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	decoded, err := Decode(command.CommandID, command.Ticker, 8, command.Type, payload, recordedAt)
	if err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if !SamePayload(command, decoded) || decoded.Create.Price != 0 || decoded.Sequence != 8 || decoded.RecordedAt != recordedAt {
		t.Fatalf("unexpected decoded market command: %#v", decoded)
	}
}

func TestValidateRejectsMarketCreateWithPrice(t *testing.T) {
	command := Command{
		CommandID: "market-command",
		Ticker:    "BTC-USD",
		Type:      CreateCommand,
		Create: &models.CreateOrderRequest{
			CommandID: "market-command",
			Ticker:    "BTC-USD",
			UserID:    "alice",
			OrderType: models.Market,
			Position:  models.Bid,
			Price:     100,
			Amount:    1,
		},
	}
	if err := Validate(command); !errors.Is(err, ErrInvalidCommand) {
		t.Fatalf("Validate want ErrInvalidCommand, got %v", err)
	}
}
