package util

import "github.com/nogie-dev/clob-trading/internal/numeric"

type PriceLevel struct {
	Price       numeric.PriceTicks
	Queue       *Queue
	Index       int
	TotalAmount numeric.QuantityLots
}
