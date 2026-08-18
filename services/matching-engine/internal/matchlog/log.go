package matchlog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/nogie-dev/clob-trading/internal/models"
)

const DefaultOutputBufferSize = 128

// MatchLog is the raw append-only execution event emitted by the matching engine.
type MatchLog struct {
	ExecutionID  string
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

// GenerateExecutionID returns a stable identity for one fill within an incoming order.
func GenerateExecutionID(ticker, takerOrderID string, matchedAt time.Time, sequence int) string {
	payload := fmt.Sprintf("%s|%s|%d|%d", ticker, takerOrderID, matchedAt.UnixNano(), sequence)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

// Store is the persistence boundary for raw match logs.
type Store interface {
	SaveMatchLog(ctx context.Context, log MatchLog) error
	SaveMatchLogs(ctx context.Context, logs []MatchLog) error
}
