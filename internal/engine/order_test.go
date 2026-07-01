package engine

import (
	"testing"

	"github.com/nogie-dev/clob-trading/internal/models"
)

func newOrder(id string, pos models.Position, price, amount float64) *models.BookOrder {
	return &models.BookOrder{
		OrderID:  id,
		Position: pos,
		Price:    price,
		Amount:   amount,
	}
}

func TestAddAndRemoveOrder(t *testing.T) {
	ob := NewOrderBook("BTC-USD")
	order := newOrder("1", models.Bid, 100, 1)

	ob.AddOrder(order)

	lvl, ok := ob.Bids[100]
	if !ok {
		t.Fatalf("price level not created")
	}
	if lvl.TotalAmount != 1 {
		t.Fatalf("TotalAmount want 1, got %v", lvl.TotalAmount)
	}
	if ob.bidLevels.Len() != 1 {
		t.Fatalf("bid heap length want 1, got %d", ob.bidLevels.Len())
	}
	if _, ok := ob.Index["1"]; !ok {
		t.Fatalf("order index not recorded")
	}

	ob.RemoveOrder("1")

	if _, ok := ob.Bids[100]; ok {
		t.Fatalf("price level should be removed after last order")
	}
	if _, ok := ob.Index["1"]; ok {
		t.Fatalf("order index should be removed")
	}
	if ob.bidLevels.Len() != 0 {
		t.Fatalf("bid heap length want 0, got %d", ob.bidLevels.Len())
	}
}

func TestEditOrderAmountIncreaseMovesToBack(t *testing.T) {
	ob := NewOrderBook("BTC-USD")
	o1 := newOrder("1", models.Bid, 100, 1)
	o2 := newOrder("2", models.Bid, 100, 1)
	ob.AddOrder(o1)
	ob.AddOrder(o2)

	// "1" 의 주문 수량 증가
	newAmt := 2.0
	req := models.EditOrderRequest{OrderID: "1", Price: 100, Amount: &newAmt}
	ob.EditOrder(req)

	lvl, ok := ob.Bids[100]
	if !ok {
		t.Fatalf("price level missing after edit")
	}

	// 최종 수량 3개
	if lvl.TotalAmount != 3 {
		t.Fatalf("TotalAmount want 3, got %v", lvl.TotalAmount)
	}

	var ids []string
	lvl.Queue.ForEach(func(v interface{}) {
		if mo, ok := v.(*models.BookOrder); ok {
			ids = append(ids, mo.OrderID)
		}
	})
	want := []string{"2", "1"}
	if len(ids) != len(want) {
		t.Fatalf("order count mismatch: got %v want %v", ids, want)
	}

	// 수량 증가 시 우선순위가 뒤로 밀리는지
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("order sequence mismatch: got %v want %v", ids, want)
		}
	}
}

func TestEditOrderPriceChangeMovesLevel(t *testing.T) {
	ob := NewOrderBook("BTC-USD")
	o1 := newOrder("1", models.Bid, 100, 1)
	ob.AddOrder(o1)

	req := models.EditOrderRequest{OrderID: "1", Price: 101}
	updated := ob.EditOrder(req)
	if updated != nil {
		ob.AddOrder(updated)
	}

	// 호가 변경 시 주문이 호가 간 이동을 하는가
	if _, ok := ob.Bids[100]; ok {
		t.Fatalf("old price level should be removed")
	}
	lvl, ok := ob.Bids[101]
	if !ok {
		t.Fatalf("new price level not created")
	}
	if lvl.TotalAmount != 1 {
		t.Fatalf("TotalAmount want 1, got %v", lvl.TotalAmount)
	}
}

func TestEditOrderAmountDecreaseKeepsOrder(t *testing.T) {
	ob := NewOrderBook("BTC-USD")
	o1 := newOrder("1", models.Bid, 100, 2)
	o2 := newOrder("2", models.Bid, 100, 1)
	ob.AddOrder(o1)
	ob.AddOrder(o2)

	// 주문 수량 감소
	newAmt := 1.0
	req := models.EditOrderRequest{OrderID: "1", Price: 100, Amount: &newAmt}
	ob.EditOrder(req)

	lvl, ok := ob.Bids[100]
	if !ok {
		t.Fatalf("price level missing after edit")
	}
	if lvl.TotalAmount != 2 {
		t.Fatalf("TotalAmount want 2, got %v", lvl.TotalAmount)
	}

	var ids []string
	lvl.Queue.ForEach(func(v interface{}) {
		if mo, ok := v.(*models.BookOrder); ok {
			ids = append(ids, mo.OrderID)
		}
	})

	// 주문 수량 감소의 경우 우선순위를 유지하는가
	want := []string{"1", "2"}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("order sequence mismatch: got %v want %v", ids, want)
		}
	}
}
