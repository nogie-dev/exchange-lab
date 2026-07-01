package engine

import (
	"log/slog"

	"github.com/nogie-dev/clob-trading/internal/models"
)

func logOrderReceived(order *models.BookOrder) {
	slog.Info("order received",
		"ticker", order.Ticker,
		"orderID", order.OrderID,
		"userID", order.UserID,
		"side", order.Position,
		"price", order.Price,
		"amount", order.Amount,
	)
}

func logOrderResting(order *models.BookOrder, reason string) {
	slog.Info("order resting",
		"reason", reason,
		"ticker", order.Ticker,
		"orderID", order.OrderID,
		"price", order.Price,
		"amount", order.Amount,
	)
}

func logOrderCancelled(order *models.BookOrder) {
	slog.Info("order cancelled",
		"ticker", order.Ticker,
		"orderID", order.OrderID,
		"userID", order.UserID,
		"side", order.Position,
		"price", order.Price,
		"amount", order.Amount,
	)
}

func logOrderEdited(order *models.BookOrder, reason string) {
	slog.Info("order edited",
		"reason", reason,
		"ticker", order.Ticker,
		"orderID", order.OrderID,
		"userID", order.UserID,
		"price", order.Price,
		"amount", order.Amount,
	)
}

func logTradeExecuted(ticker, takerID, makerID string, price, amount float64) {
	slog.Info("trade executed",
		"ticker", ticker,
		"takerOrderID", takerID,
		"makerOrderID", makerID,
		"price", price,
		"amount", amount,
	)
}
