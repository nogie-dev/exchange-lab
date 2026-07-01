package engine

import (
	"container/heap"
	"container/list"
	"fmt"
	"log/slog"
	"time"

	"github.com/nogie-dev/clob-trading/internal/models"
	"github.com/nogie-dev/clob-trading/internal/util"
)

type OrderBook struct {
	Bids      map[float64]*util.PriceLevel
	Asks      map[float64]*util.PriceLevel
	bidLevels util.MaxPriceHeap
	askLevels util.MinPriceHeap
	Index     map[string]*list.Element
	Ticker    string
}

func NewOrderBook(ticker string) *OrderBook {
	ob := &OrderBook{
		Ticker: ticker,
		Bids:   make(map[float64]*util.PriceLevel),
		Asks:   make(map[float64]*util.PriceLevel),
		Index:  make(map[string]*list.Element),
	}
	heap.Init(&ob.bidLevels)
	heap.Init(&ob.askLevels)
	return ob
}

func (ob *OrderBook) side(order *models.BookOrder) (map[float64]*util.PriceLevel, heap.Interface, bool) {
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
	return models.BookOrder{
		OrderID:   util.GenerateOrderID(req),
		Ticker:    req.Ticker,
		UserID:    req.UserID,
		OrderType: req.OrderType,
		Position:  req.Position,
		Price:     req.Price,
		Amount:    req.Amount,
		Status:    models.Pending,
		Timestamp: time.Now(),
		Nonce:     req.Nonce,
	}
}

func (ob *OrderBook) AddOrder(order *models.BookOrder) {
	var levels map[float64]*util.PriceLevel
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

func (ob *OrderBook) level(order *models.BookOrder) (*util.PriceLevel, map[float64]*util.PriceLevel, heap.Interface, bool) {
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

func (ob *OrderBook) RemoveOrder(orderID string) {
	elem, ok := ob.Index[orderID]
	if !ok || elem == nil {
		slog.Warn("order not found in index", "orderID", orderID)
		return
	}

	current, ok := elem.Value.(*models.BookOrder)
	if !ok || current == nil {
		slog.Error("order type mismatch", "orderID", orderID)
		return
	}

	lvl, levels, h, ok := ob.level(current)
	if !ok {
		return
	}

	ob.removeElement(lvl, levels, h, elem, current.Amount)
	logOrderCancelled(current)
}

func (ob *OrderBook) removeElement(lvl *util.PriceLevel, levels map[float64]*util.PriceLevel, h heap.Interface, elem *list.Element, fallbackAmount float64) {
	removed := lvl.Queue.Remove(elem)

	var orderID string
	if mo, ok := elem.Value.(*models.BookOrder); ok && mo != nil {
		orderID = mo.OrderID
	}
	if orderID != "" {
		delete(ob.Index, orderID)
	}

	var amt float64
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

func (ob *OrderBook) EditOrder(req models.EditOrderRequest) *models.BookOrder {
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

	if priceChanged {
		// 기존 레벨에서 제거, 업데이트된 주문 반환 (매칭은 bookworker에서)
		ob.removeElement(lvl, levels, h, elem, existing.Amount)
		existing.Price = req.Price
		if req.Amount != nil {
			existing.Amount = *req.Amount
		}
		existing.Timestamp = time.Now()
		logOrderEdited(existing, "price_changed")
		return existing
	}

	if amountChanged {
		delta := *req.Amount - existing.Amount
		if delta > 0 {
			// 수량 증가: 우선순위 리셋을 위해 제거 후 재삽입
			ob.removeElement(lvl, levels, h, elem, existing.Amount)
			existing.Amount = *req.Amount
			existing.Timestamp = time.Now()
			ob.AddOrder(existing)
			logOrderEdited(existing, "amount_increased")
		} else {
			// 수량 감소: 위치 유지, 누적만 반영
			existing.Amount = *req.Amount
			existing.Timestamp = time.Now()
			lvl.TotalAmount += delta
			logOrderEdited(existing, "amount_decreased")
		}
	}

	return nil
}

func (ob *OrderBook) PrintOrderBook() {
	bidHeap := append(util.MaxPriceHeap(nil), ob.bidLevels...)
	heap.Init(&bidHeap)
	for bidHeap.Len() > 0 {
		lvl := heap.Pop(&bidHeap).(*util.PriceLevel)
		fmt.Printf("BID price=%.4f total=%.4f\n", lvl.Price, lvl.TotalAmount)
	}

	askHeap := append(util.MinPriceHeap(nil), ob.askLevels...)
	heap.Init(&askHeap)
	for askHeap.Len() > 0 {
		lvl := heap.Pop(&askHeap).(*util.PriceLevel)
		fmt.Printf("ASK price=%.4f total=%.4f\n", lvl.Price, lvl.TotalAmount)
	}
}

// dropPriceLevel removes an empty price level from heap and map.
func (ob *OrderBook) dropPriceLevel(h heap.Interface, levels map[float64]*util.PriceLevel, lvl *util.PriceLevel) {
	if lvl == nil {
		return
	}
	if lvl.Index >= 0 && lvl.Index < h.Len() {
		heap.Remove(h, lvl.Index)
	}
	delete(levels, lvl.Price)
}
