package matchlog

import (
	"context"
	"time"

	"github.com/nogie-dev/clob-trading/internal/models"
)

// MatchLog is the raw append-only execution event emitted by the matching engine.
type MatchLog struct {
	Ticker       string
	Price        float64
	Amount       float64
	QuoteAmount  float64
	MakerOrderID string
	TakerOrderID string
	MakerUserID  string
	TakerUserID  string
	MakerSide    models.Position
	TakerSide    models.Position
	MatchedAt    time.Time
}

// Store is the persistence boundary for raw match logs.
type Store interface {
	SaveMatchLog(ctx context.Context, log MatchLog) error
	SaveMatchLogs(ctx context.Context, logs []MatchLog) error
}
