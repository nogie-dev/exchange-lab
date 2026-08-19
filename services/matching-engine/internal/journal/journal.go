package journal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/nogie-dev/clob-trading/internal/models"
	"github.com/nogie-dev/clob-trading/internal/numeric"
)

var (
	ErrInvalidCommand   = errors.New("invalid journal command")
	ErrCommandConflict  = errors.New("journal command id conflict")
	ErrStoreUnavailable = errors.New("order journal store unavailable")
)

type CommandType string

const (
	CreateCommand CommandType = "CREATE"
	AmendCommand  CommandType = "AMEND"
	CancelCommand CommandType = "CANCEL"
)

type Command struct {
	CommandID           string
	Ticker              string
	Sequence            int64
	Type                CommandType
	RecordedAt          time.Time
	MarketConfigVersion int64
	Create              *models.CreateOrderRequest
	Amend               *models.EditOrderRequest
	Cancel              *models.CancelOrderRequest
}

type AppendResult struct {
	Command  Command
	Inserted bool
}

type Store interface {
	Append(ctx context.Context, command Command) (AppendResult, error)
	List(ctx context.Context) ([]Command, error)
}

func EncodePayload(command Command) ([]byte, error) {
	if err := Validate(command); err != nil {
		return nil, err
	}
	switch command.Type {
	case CreateCommand:
		return json.Marshal(command.Create)
	case AmendCommand:
		return json.Marshal(command.Amend)
	case CancelCommand:
		return json.Marshal(command.Cancel)
	default:
		return nil, fmt.Errorf("%w: unsupported type %q", ErrInvalidCommand, command.Type)
	}
}

func Decode(commandID, ticker string, sequence int64, commandType CommandType, payload []byte, recordedAt time.Time) (Command, error) {
	return DecodeWithVersion(commandID, ticker, sequence, commandType, payload, recordedAt, numeric.DefaultConfigVersion)
}

func DecodeWithVersion(commandID, ticker string, sequence int64, commandType CommandType, payload []byte, recordedAt time.Time, configVersion int64) (Command, error) {
	command := Command{
		CommandID:           commandID,
		Ticker:              ticker,
		Sequence:            sequence,
		Type:                commandType,
		RecordedAt:          recordedAt,
		MarketConfigVersion: configVersion,
	}
	var destination any
	switch commandType {
	case CreateCommand:
		command.Create = &models.CreateOrderRequest{}
		destination = command.Create
	case AmendCommand:
		command.Amend = &models.EditOrderRequest{}
		destination = command.Amend
	case CancelCommand:
		command.Cancel = &models.CancelOrderRequest{}
		destination = command.Cancel
	default:
		return Command{}, fmt.Errorf("%w: unsupported type %q", ErrInvalidCommand, commandType)
	}
	if err := json.Unmarshal(payload, destination); err != nil {
		return Command{}, fmt.Errorf("decode journal payload: %w", err)
	}
	if err := Validate(command); err != nil {
		return Command{}, err
	}
	return command, nil
}

func Validate(command Command) error {
	switch {
	case command.CommandID == "":
		return fmt.Errorf("%w: command id is required", ErrInvalidCommand)
	case command.Ticker == "":
		return fmt.Errorf("%w: ticker is required", ErrInvalidCommand)
	case command.MarketConfigVersion < 0:
		return fmt.Errorf("%w: market config version cannot be negative", ErrInvalidCommand)
	}

	var payloadCommandID, payloadTicker string
	switch command.Type {
	case CreateCommand:
		if command.Create == nil || command.Amend != nil || command.Cancel != nil {
			return fmt.Errorf("%w: create payload is required", ErrInvalidCommand)
		}
		payloadCommandID, payloadTicker = command.Create.CommandID, command.Create.Ticker
		validPrice := false
		switch command.Create.OrderType {
		case models.Limit:
			validPrice = command.Create.Price > 0
		case models.Market:
			validPrice = command.Create.Price == 0
		}
		if strings.TrimSpace(command.Create.UserID) == "" ||
			(command.Create.OrderType != models.Limit && command.Create.OrderType != models.Market) ||
			(command.Create.Position != models.Bid && command.Create.Position != models.Ask) ||
			!validPrice || command.Create.Amount <= 0 {
			return fmt.Errorf("%w: invalid create payload", ErrInvalidCommand)
		}
	case AmendCommand:
		if command.Amend == nil || command.Create != nil || command.Cancel != nil {
			return fmt.Errorf("%w: amend payload is required", ErrInvalidCommand)
		}
		payloadCommandID, payloadTicker = command.Amend.CommandID, command.Amend.Ticker
		if strings.TrimSpace(command.Amend.OrderID) == "" || command.Amend.Price <= 0 ||
			(command.Amend.Amount != nil && *command.Amend.Amount <= 0) {
			return fmt.Errorf("%w: invalid amend payload", ErrInvalidCommand)
		}
	case CancelCommand:
		if command.Cancel == nil || command.Create != nil || command.Amend != nil {
			return fmt.Errorf("%w: cancel payload is required", ErrInvalidCommand)
		}
		payloadCommandID, payloadTicker = command.Cancel.CommandID, command.Cancel.Ticker
		if strings.TrimSpace(command.Cancel.OrderID) == "" {
			return fmt.Errorf("%w: invalid cancel payload", ErrInvalidCommand)
		}
	default:
		return fmt.Errorf("%w: unsupported type %q", ErrInvalidCommand, command.Type)
	}
	if payloadCommandID != command.CommandID {
		return fmt.Errorf("%w: payload command id mismatch", ErrInvalidCommand)
	}
	if payloadTicker != command.Ticker {
		return fmt.Errorf("%w: payload ticker mismatch", ErrInvalidCommand)
	}
	return nil
}

func SamePayload(left, right Command) bool {
	return left.CommandID == right.CommandID &&
		left.Ticker == right.Ticker &&
		left.Type == right.Type &&
		effectiveConfigVersion(left) == effectiveConfigVersion(right) &&
		reflect.DeepEqual(left.Create, right.Create) &&
		reflect.DeepEqual(left.Amend, right.Amend) &&
		reflect.DeepEqual(left.Cancel, right.Cancel)
}

func effectiveConfigVersion(command Command) int64 {
	if command.MarketConfigVersion == 0 {
		return numeric.DefaultConfigVersion
	}
	return command.MarketConfigVersion
}
