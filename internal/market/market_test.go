package market

import (
	"errors"
	"testing"
)

func TestNormalizeTickerTrimsInput(t *testing.T) {
	ticker, err := NormalizeTicker(" ETH-USD ")
	if err != nil {
		t.Fatalf("NormalizeTicker returned error: %v", err)
	}
	if ticker != "ETH-USD" {
		t.Fatalf("ticker want ETH-USD, got %q", ticker)
	}
}

func TestNormalizeTickerRejectsEmptyInput(t *testing.T) {
	_, err := NormalizeTicker(" ")
	if !errors.Is(err, ErrInvalidTicker) {
		t.Fatalf("NormalizeTicker want ErrInvalidTicker, got %v", err)
	}
}
