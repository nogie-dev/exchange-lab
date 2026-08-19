package engine

import (
	"strconv"

	"github.com/nogie-dev/clob-trading/internal/numeric"
)

func testPrice(value float64) numeric.PriceTicks {
	parsed, err := numeric.ParseScaled(strconv.FormatFloat(value, 'f', -1, 64), numeric.DefaultPriceScale)
	if err != nil {
		panic(err)
	}
	return numeric.PriceTicks(parsed)
}

func testQuantity(value float64) numeric.QuantityLots {
	parsed, err := numeric.ParseScaled(strconv.FormatFloat(value, 'f', -1, 64), numeric.DefaultQuantityScale)
	if err != nil {
		panic(err)
	}
	return numeric.QuantityLots(parsed)
}

func testQuote(value float64) numeric.QuoteAtoms {
	parsed, err := numeric.ParseScaled(strconv.FormatFloat(value, 'f', -1, 64), numeric.DefaultQuoteScale)
	if err != nil {
		panic(err)
	}
	return numeric.QuoteAtoms(parsed)
}

func approxEqual(got numeric.QuantityLots, want any) bool {
	switch value := want.(type) {
	case float64:
		return got == testQuantity(value)
	case numeric.QuantityLots:
		return got == value
	case int:
		return got == testQuantity(float64(value))
	default:
		panic("unsupported quantity comparison")
	}
}
