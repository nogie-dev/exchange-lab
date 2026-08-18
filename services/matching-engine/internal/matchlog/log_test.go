package matchlog

import (
	"testing"
	"time"
)

func TestGenerateExecutionIDIsStableAndSequenceSpecific(t *testing.T) {
	matchedAt := time.Date(2026, 7, 17, 12, 0, 0, 123, time.UTC)

	first := GenerateExecutionID("BTC-USD", "order-1", matchedAt, 0)
	if first == "" {
		t.Fatal("execution ID must not be empty")
	}
	if got := GenerateExecutionID("BTC-USD", "order-1", matchedAt, 0); got != first {
		t.Fatalf("execution ID must be stable: want %q, got %q", first, got)
	}
	if next := GenerateExecutionID("BTC-USD", "order-1", matchedAt, 1); next == first {
		t.Fatal("different fill sequences must have different execution IDs")
	}
}
