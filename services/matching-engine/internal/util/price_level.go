package util

type PriceLevel struct {
	Price       float64
	Queue       *Queue
	Index       int
	TotalAmount float64
}
