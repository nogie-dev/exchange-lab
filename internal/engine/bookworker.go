package engine

import (
	"context"
	"log/slog"

	"github.com/nogie-dev/clob-trading/internal/matchlog"
)

type BookWorker struct {
	ticker        string
	OrderBook     *OrderBook
	in            chan Event
	matchLogStore matchlog.Store
}

// NewBookWorker owns one orderbook per ticker and consumes events over its input channel.
// If an orderbook is provided, the worker will reuse it; otherwise a new one is created.
func NewBookWorker(ticker string, ob *OrderBook) *BookWorker {
	return NewBookWorkerWithMatchLogStore(ticker, ob, nil)
}

func NewBookWorkerWithMatchLogStore(ticker string, ob *OrderBook, store matchlog.Store) *BookWorker {
	if ob == nil {
		ob = NewOrderBook(ticker)
	}
	return &BookWorker{
		ticker:        ticker,
		OrderBook:     ob,
		in:            make(chan Event, 128),
		matchLogStore: store,
	}
}

// Run processes events until the channel is closed.
func (w *BookWorker) Run() {
	for ev := range w.in {
		// Basic ticker guard to avoid misroutes.
		if ev.Ticker != "" && ev.Ticker != w.ticker {
			slog.Warn("mismatched ticker", "got", ev.Ticker, "worker", w.ticker)
			continue
		}

		switch ev.Type {
		case NewOrder:
			if ev.NewOrder == nil {
				slog.Warn("nil NewOrder payload", "ticker", w.ticker)
				continue
			}
			if ev.NewOrder.Ticker != w.ticker {
				slog.Warn("mismatched NewOrder payload ticker",
					"eventTicker", ev.Ticker,
					"payloadTicker", ev.NewOrder.Ticker,
					"worker", w.ticker,
				)
				continue
			}
			order := CreateOrder(*ev.NewOrder)
			originalAmount := order.Amount
			logOrderReceived(&order)
			result := Match(w.OrderBook, &order)
			w.saveMatchLogs(result.Logs)
			if result.Residual != nil {
				w.OrderBook.AddOrder(result.Residual)
				reason := "no_match"
				if result.Residual.Amount < originalAmount {
					reason = "partial_fill"
				}
				logOrderResting(result.Residual, reason)
			}
		case CancelOrder:
			if ev.CancelReq == nil {
				slog.Warn("nil CancelRequest payload", "ticker", w.ticker)
				continue
			}
			if ev.CancelReq.Ticker != w.ticker {
				slog.Warn("mismatched CancelOrder payload ticker",
					"eventTicker", ev.Ticker,
					"payloadTicker", ev.CancelReq.Ticker,
					"worker", w.ticker,
					"orderID", ev.CancelReq.OrderID,
				)
				continue
			}
			w.OrderBook.RemoveOrder(ev.CancelReq.OrderID)
		case EditOrder:
			if ev.EditReq == nil {
				slog.Warn("nil EditOrderRequest payload", "ticker", w.ticker)
				continue
			}
			if ev.EditReq.Ticker != w.ticker {
				slog.Warn("mismatched EditOrder payload ticker",
					"eventTicker", ev.Ticker,
					"payloadTicker", ev.EditReq.Ticker,
					"worker", w.ticker,
					"orderID", ev.EditReq.OrderID,
				)
				continue
			}
			updated := w.OrderBook.EditOrder(*ev.EditReq)
			if updated != nil {
				originalAmount := updated.Amount
				result := Match(w.OrderBook, updated)
				w.saveMatchLogs(result.Logs)
				if result.Residual != nil {
					w.OrderBook.AddOrder(result.Residual)
					reason := "no_match"
					if result.Residual.Amount < originalAmount {
						reason = "partial_fill"
					}
					logOrderResting(result.Residual, reason)
				}
			}
		default:
			slog.Warn("unsupported event type", "type", ev.Type)
		}
	}
}

func (w *BookWorker) saveMatchLogs(logs []matchlog.MatchLog) {
	if len(logs) == 0 || w.matchLogStore == nil {
		return
	}
	if err := w.matchLogStore.SaveMatchLogs(context.Background(), logs); err != nil {
		// ponytail: durability policy is unresolved; decide retry/backpressure/drop before production use.
		slog.Error("match log store failed", "ticker", w.ticker, "error", err)
	}
}
