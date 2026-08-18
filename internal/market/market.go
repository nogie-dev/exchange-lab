package market

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidTicker    = errors.New("invalid market ticker")
	ErrStoreUnavailable = errors.New("market store unavailable")
)

type Market struct {
	Ticker    string
	CreatedAt time.Time
}

type AddResult struct {
	Market   Market
	Inserted bool
}

type Store interface {
	Add(ctx context.Context, ticker string) (AddResult, error)
	List(ctx context.Context) ([]Market, error)
}

func NormalizeTicker(ticker string) (string, error) {
	ticker = strings.TrimSpace(ticker)
	if ticker == "" {
		return "", fmt.Errorf("%w: ticker is required", ErrInvalidTicker)
	}
	return ticker, nil
}
