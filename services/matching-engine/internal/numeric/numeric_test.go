package numeric

import (
	"math"
	"testing"
)

func TestParseScaledCanonicalizesDecimalStrings(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		want  int64
		scale int32
	}{
		{name: "integer", raw: "100", want: 10000000000, scale: 8},
		{name: "trailing zeros", raw: "100.00", want: 10000000000, scale: 8},
		{name: "leading zero", raw: ".25", want: 25000000, scale: 8},
		{name: "negative", raw: "-0.5", want: -50000000, scale: 8},
		{name: "zero", raw: "0.000", want: 0, scale: 8},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseScaled(test.raw, test.scale)
			if err != nil {
				t.Fatalf("ParseScaled returned error: %v", err)
			}
			if got != test.want {
				t.Fatalf("ParseScaled want %d, got %d", test.want, got)
			}
		})
	}
}

func TestParseScaledRejectsAmbiguousOrLossyInput(t *testing.T) {
	for _, raw := range []string{"", ".", "1e2", "1.234", "1.2.3", "-", "abc", "+"} {
		if _, err := ParseScaled(raw, 2); err == nil {
			t.Fatalf("ParseScaled(%q) should fail", raw)
		}
	}
	if _, err := ParseScaled("92233720369", 8); err == nil {
		t.Fatal("overflowing scaled value should fail")
	}
}

func TestPrecisionValidatesTickAndLotUnits(t *testing.T) {
	precision := DefaultPrecision()
	precision.TickSizeUnits = 100
	precision.LotSizeUnits = 1000

	if _, err := precision.ParsePrice("1.00000001"); err == nil {
		t.Fatal("price not aligned to tick should fail")
	}
	if got, err := precision.ParsePrice("1.000001"); err != nil || got != 100000100 {
		t.Fatalf("aligned price parse got %v, %v", got, err)
	}
	if _, err := precision.ParseQuantity("0.00000001"); err == nil {
		t.Fatal("quantity not aligned to lot should fail")
	}
	if got, err := precision.ParseQuantity("0.00001"); err != nil || got != 1000 {
		t.Fatalf("aligned quantity parse got %v, %v", got, err)
	}
}

func TestQuoteAmountUsesCheckedFixedPointArithmetic(t *testing.T) {
	precision := DefaultPrecision()
	got, err := precision.QuoteAmount(MustPrice("100"), MustQuantity("0.25"))
	if err != nil {
		t.Fatalf("QuoteAmount returned error: %v", err)
	}
	if got != MustQuote("25") {
		t.Fatalf("QuoteAmount want %d, got %d", MustQuote("25"), got)
	}

	precision.PriceScale = 18
	precision.QuantityScale = 18
	precision.QuoteScale = 18
	if _, err := precision.QuoteAmount(PriceTicks(math.MaxInt64), QuantityLots(math.MaxInt64)); err == nil {
		t.Fatal("overflowing quote amount should fail")
	}
}

func TestFormatScaledRemovesNonCanonicalTrailingZeros(t *testing.T) {
	if got := FormatScaled(10000000000, 8); got != "100" {
		t.Fatalf("FormatScaled want 100, got %q", got)
	}
	if got := FormatScaled(25000000, 8); got != "0.25" {
		t.Fatalf("FormatScaled want 0.25, got %q", got)
	}
}
