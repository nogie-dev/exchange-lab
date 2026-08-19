package engine

import (
	"container/heap"
	"container/list"
	"log/slog"
	"sort"
	"time"

	"github.com/nogie-dev/clob-trading/internal/models"
	"github.com/nogie-dev/clob-trading/internal/numeric"
	"github.com/nogie-dev/clob-trading/internal/util"
)

type OrderBook struct {
	Bids      map[numeric.PriceTicks]*util.PriceLevel
	Asks      map[numeric.PriceTicks]*util.PriceLevel
	bidLevels util.MaxPriceHeap
	askLevels util.MinPriceHeap
	Index     map[string]*list.Element
	Ticker    string
	Precision numeric.Precision
}

type OrderBookSnapshot struct {
	Ticker string           `json:"ticker"`
	Bids   []OrderBookLevel `json:"bids"`
	Asks   []OrderBookLevel `json:"asks"`
}

type OrderBookLevel struct {
	Price            numeric.PriceTicks   `json:"price"`
	Amount           numeric.QuantityLots `json:"amount"`
	CumulativeAmount numeric.QuantityLots `json:"cumulativeAmount"`
}

// EditOrderResult captures the immutable state immediately before and after a
// successful amendment. RequiresRematch is true when the amended order was
// detached from the book because its price changed.
type EditOrderResult struct {
	Before          models.BookOrder
	After           models.BookOrder
	RequiresRematch bool
}

func NewOrderBook(ticker string) *OrderBook {
	return NewOrderBookWithPrecision(ticker, numeric.DefaultPrecision())
}

func NewOrderBookWithPrecision(ticker string, precision numeric.Precision) *OrderBook {
	precision = precision.WithDefaults()
	ob := &OrderBook{
		Ticker:    ticker,
		Precision: precision,
		Bids:      make(map[numeric.PriceTicks]*util.PriceLevel),
		Asks:      make(map[numeric.PriceTicks]*util.PriceLevel),
		Index:     make(map[string]*list.Element),
	}
	heap.Init(&ob.bidLevels)
	heap.Init(&ob.askLevels)
	return ob
}

func (ob *OrderBook) side(order *models.BookOrder) (map[numeric.PriceTicks]*util.PriceLevel, heap.Interface, bool) {
	switch order.Position {
	case models.Bid:
		return ob.Bids, &ob.bidLevels, true
	case models.Ask:
		return ob.Asks, &ob.askLevels, true
	default:
		return nil, nil, false
	}
}

func CreateOrder(req models.CreateOrderRequest) models.BookOrder {
	return CreateOrderAt(req, time.Now())
}

func CreateOrderAt(req models.CreateOrderRequest, recordedAt time.Time) models.BookOrder {
	return models.BookOrder{
		OrderID:   util.GenerateOrderID(req),
		Ticker:    req.Ticker,
		UserID:    req.UserID,
		OrderType: req.OrderType,
		Position:  req.Position,
		Price:     req.Price,
		Amount:    req.Amount,
		Status:    models.Pending,
		Timestamp: recordedAt,
		Nonce:     req.Nonce,
	}
}

func (ob *OrderBook) AddOrder(order *models.BookOrder) {
	var levels map[numeric.PriceTicks]*util.PriceLevel
	var h heap.Interface
	switch order.Position {
	case models.Bid:
		levels, h = ob.Bids, &ob.bidLevels
	case models.Ask:
		levels, h = ob.Asks, &ob.askLevels
	default:
		return
	}

	lvl, ok := levels[order.Price]
	// 해당 호가에 존재하지 않으면 호가 생성
	if !ok {
		lvl = &util.PriceLevel{Price: order.Price, Queue: util.NewQueue()}
		levels[order.Price] = lvl
		heap.Push(h, lvl)
	}
	lvl.TotalAmount += order.Amount
	ob.Index[order.OrderID] = lvl.Queue.Push(order)
}

func (ob *OrderBook) level(order *models.BookOrder) (*util.PriceLevel, map[numeric.PriceTicks]*util.PriceLevel, heap.Interface, bool) {
	levels, h, ok := ob.side(order)
	if !ok {
		slog.Error("unsupported position", "position", order.Position)
		return nil, nil, nil, false
	}
	lvl, ok := levels[order.Price]
	if !ok || lvl == nil {
		slog.Error("price level not found", "price", order.Price)
		return nil, nil, nil, false
	}
	return lvl, levels, h, true
}

func (ob *OrderBook) RemoveOrder(orderID string) *models.BookOrder {
	elem, ok := ob.Index[orderID]
	if !ok || elem == nil {
		slog.Warn("order not found in index", "orderID", orderID)
		return nil
	}

	current, ok := elem.Value.(*models.BookOrder)
	if !ok || current == nil {
		slog.Error("order type mismatch", "orderID", orderID)
		return nil
	}

	lvl, levels, h, ok := ob.level(current)
	if !ok {
		return nil
	}

	removed := *current
	ob.removeElement(lvl, levels, h, elem, current.Amount)
	logOrderCancelled(current)
	return &removed
}

func (ob *OrderBook) removeElement(lvl *util.PriceLevel, levels map[numeric.PriceTicks]*util.PriceLevel, h heap.Interface, elem *list.Element, fallbackAmount numeric.QuantityLots) {
	removed := lvl.Queue.Remove(elem)

	var orderID string
	if mo, ok := elem.Value.(*models.BookOrder); ok && mo != nil {
		orderID = mo.OrderID
	}
	if orderID != "" {
		delete(ob.Index, orderID)
	}

	var amt numeric.QuantityLots
	if mo, ok := removed.(*models.BookOrder); ok && mo != nil {
		amt = mo.Amount
	} else {
		amt = fallbackAmount
	}
	lvl.TotalAmount -= amt

	// 큐에 주문이 없을 경우 삭제
	if lvl.Queue.Len() == 0 {
		if lvl.Index >= 0 && lvl.Index < h.Len() {
			heap.Remove(h, lvl.Index)
		}
		delete(levels, lvl.Price)
	}
}

func (ob *OrderBook) EditOrder(req models.EditOrderRequest) *EditOrderResult {
	return ob.EditOrderAt(req, time.Now())
}

func (ob *OrderBook) EditOrderAt(req models.EditOrderRequest, recordedAt time.Time) *EditOrderResult {
	elem, ok := ob.Index[req.OrderID]
	if !ok || elem == nil {
		slog.Warn("order not found", "orderID", req.OrderID)
		return nil
	}

	existing, ok := elem.Value.(*models.BookOrder)
	if !ok || existing == nil {
		slog.Error("order type mismatch", "orderID", req.OrderID)
		return nil
	}

	lvl, levels, h, ok := ob.level(&models.BookOrder{Position: existing.Position, Price: existing.Price})
	if !ok {
		return nil
	}

	priceChanged := existing.Price != req.Price
	amountChanged := req.Amount != nil && *req.Amount != existing.Amount
	if !priceChanged && !amountChanged {
		return nil
	}
	before := *existing

	if priceChanged {
		// 기존 레벨에서 제거, 업데이트된 주문 반환 (매칭은 bookworker에서)
		ob.removeElement(lvl, levels, h, elem, existing.Amount)
		existing.Price = req.Price
		if req.Amount != nil {
			existing.Amount = *req.Amount
		}
		existing.Timestamp = recordedAt
		logOrderEdited(existing, "price_changed")
		return &EditOrderResult{
			Before:          before,
			After:           *existing,
			RequiresRematch: true,
		}
	}

	delta := *req.Amount - existing.Amount
	if delta > 0 {
		// 수량 증가: 우선순위 리셋을 위해 제거 후 재삽입
		ob.removeElement(lvl, levels, h, elem, existing.Amount)
		existing.Amount = *req.Amount
		existing.Timestamp = recordedAt
		ob.AddOrder(existing)
		logOrderEdited(existing, "amount_increased")
	} else {
		// 수량 감소: 위치 유지, 누적만 반영
		existing.Amount = *req.Amount
		existing.Timestamp = recordedAt
		lvl.TotalAmount += delta
		logOrderEdited(existing, "amount_decreased")
	}

	return &EditOrderResult{Before: before, After: *existing}
}

func (ob *OrderBook) Snapshot(depth int) OrderBookSnapshot {
	return OrderBookSnapshot{
		Ticker: ob.Ticker,
		Bids:   snapshotLevels(ob.Bids, depth, true),
		Asks:   snapshotLevels(ob.Asks, depth, false),
	}
}

func snapshotLevels(levels map[numeric.PriceTicks]*util.PriceLevel, depth int, desc bool) []OrderBookLevel {
	out := make([]OrderBookLevel, 0, len(levels))
	for _, lvl := range levels {
		out = append(out, OrderBookLevel{
			Price:  lvl.Price,
			Amount: lvl.TotalAmount,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if desc {
			return out[i].Price > out[j].Price
		}
		return out[i].Price < out[j].Price
	})

	if depth > 0 && depth < len(out) {
		out = out[:depth]
	}

	var cumulative numeric.QuantityLots
	for i := range out {
		cumulative += out[i].Amount
		out[i].CumulativeAmount = cumulative
	}
	return out
}

// dropPriceLevel removes an empty price level from heap and map.
func (ob *OrderBook) dropPriceLevel(h heap.Interface, levels map[numeric.PriceTicks]*util.PriceLevel, lvl *util.PriceLevel) {
	if lvl == nil {
		return
	}
	if lvl.Index >= 0 && lvl.Index < h.Len() {
		heap.Remove(h, lvl.Index)
	}
	delete(levels, lvl.Price)
}
