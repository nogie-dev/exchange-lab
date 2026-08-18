package engine

import (
	"log/slog"
	"math"

	"github.com/nogie-dev/clob-trading/internal/matchlog"
	"github.com/nogie-dev/clob-trading/internal/models"
)

type MatchResult struct {
	Residual         *models.BookOrder
	Logs             []matchlog.MatchLog
	MakerTransitions []MakerFillTransition
}

// MakerFillTransition captures one maker's quantity change for one execution.
// Order is the immutable pre-execution snapshot used for order identity and
// static metadata. The caller derives the incoming taker's aggregate outcome
// separately from its original and residual amounts.
type MakerFillTransition struct {
	Order           models.BookOrder
	PreviousAmount  float64
	FilledAmount    float64
	RemainingAmount float64
}

// Match consumes an incoming order against the provided orderbook.
// Limit orders execute only while their price crosses the best opposite level.
// Market orders consume the best opposite levels without a price limit. Any
// residual is returned to the caller, which decides whether it may rest.
func Match(book *OrderBook, incoming *models.BookOrder) MatchResult {
	if book == nil || incoming == nil {
		return MatchResult{Residual: incoming}
	}

	result := MatchResult{}

	// 오더북에 등록된 주문과 최신 주문 비교
	switch incoming.Position {
	case models.Bid:
		for {
			bestAsk := book.askLevels.Peek()
			if bestAsk == nil || incoming.Amount <= 0 || !canMatchAtPrice(incoming, bestAsk.Price) {
				break
			}
			elem := bestAsk.Queue.Front()
			if elem == nil {
				break
			}
			target, ok := elem.Value.(*models.BookOrder)
			if !ok || target == nil {
				slog.Error("unexpected queue element type", "value", elem.Value)
				break
			}

			before := *target
			tradeAmt := math.Min(incoming.Amount, target.Amount)
			logTradeExecuted(incoming.Ticker, incoming.OrderID, target.OrderID, bestAsk.Price, tradeAmt)
			result.Logs = append(result.Logs, newMatchLog(book.Ticker, incoming, target, bestAsk.Price, tradeAmt, len(result.Logs)))
			incoming.Amount -= tradeAmt
			target.Amount -= tradeAmt
			bestAsk.TotalAmount -= tradeAmt
			result.MakerTransitions = append(result.MakerTransitions, MakerFillTransition{
				Order:           before,
				PreviousAmount:  before.Amount,
				FilledAmount:    tradeAmt,
				RemainingAmount: target.Amount,
			})

			if target.Amount <= 0 {
				bestAsk.Queue.Remove(elem)
				delete(book.Index, target.OrderID)
			}
			if bestAsk.Queue.Len() == 0 {
				book.dropPriceLevel(&book.askLevels, book.Asks, bestAsk)
			}
		}

	case models.Ask:
		for {
			bestBid := book.bidLevels.Peek()
			if bestBid == nil || incoming.Amount <= 0 || !canMatchAtPrice(incoming, bestBid.Price) {
				break
			}
			elem := bestBid.Queue.Front()
			if elem == nil {
				break
			}
			target, ok := elem.Value.(*models.BookOrder)
			if !ok || target == nil {
				slog.Error("unexpected queue element type", "value", elem.Value)
				break
			}

			before := *target
			tradeAmt := math.Min(incoming.Amount, target.Amount)
			logTradeExecuted(incoming.Ticker, incoming.OrderID, target.OrderID, bestBid.Price, tradeAmt)
			result.Logs = append(result.Logs, newMatchLog(book.Ticker, incoming, target, bestBid.Price, tradeAmt, len(result.Logs)))
			incoming.Amount -= tradeAmt
			target.Amount -= tradeAmt
			bestBid.TotalAmount -= tradeAmt
			result.MakerTransitions = append(result.MakerTransitions, MakerFillTransition{
				Order:           before,
				PreviousAmount:  before.Amount,
				FilledAmount:    tradeAmt,
				RemainingAmount: target.Amount,
			})

			if target.Amount <= 0 {
				bestBid.Queue.Remove(elem)
				delete(book.Index, target.OrderID)
			}
			if bestBid.Queue.Len() == 0 {
				book.dropPriceLevel(&book.bidLevels, book.Bids, bestBid)
			}
		}
	}

	if incoming.Amount <= 0 {
		return result
	}
	result.Residual = incoming
	return result
}

func canMatchAtPrice(incoming *models.BookOrder, oppositePrice float64) bool {
	if incoming.OrderType == models.Market {
		return true
	}

	if incoming.Position == models.Bid {
		return incoming.Price >= oppositePrice
	}
	return incoming.Price <= oppositePrice
}

func newMatchLog(ticker string, taker, maker *models.BookOrder, price, amount float64, sequence int) matchlog.MatchLog {
	return matchlog.MatchLog{
		ExecutionID:  matchlog.GenerateExecutionID(ticker, taker.OrderID, taker.Timestamp, sequence),
		Ticker:       ticker,
		Price:        price,
		Amount:       amount,
		QuoteAmount:  price * amount,
		MakerOrderID: maker.OrderID,
		TakerOrderID: taker.OrderID,
		MakerUserID:  maker.UserID,
		TakerUserID:  taker.UserID,
		MakerSide:    maker.Position,
		TakerSide:    taker.Position,
		MatchedAt:    taker.Timestamp,
	}
}
