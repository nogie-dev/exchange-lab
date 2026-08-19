package engine

import (
	"testing"

	"github.com/nogie-dev/clob-trading/internal/models"
)

func newOrder(id string, pos models.Position, price, amount float64) *models.BookOrder {
	return &models.BookOrder{
		Ticker:    "BTC-USD",
		OrderID:   id,
		UserID:    "test-user",
		OrderType: models.Limit,
		Position:  pos,
		Price:     testPrice(price),
		Amount:    testQuantity(amount),
	}
}

func TestAddAndRemoveOrder(t *testing.T) {
	ob := NewOrderBook("BTC-USD")
	order := newOrder("1", models.Bid, 100, 1)

	ob.AddOrder(order)

	lvl, ok := ob.Bids[testPrice(100)]
	if !ok {
		t.Fatalf("price level not created")
	}
	if lvl.TotalAmount != testQuantity(1) {
		t.Fatalf("TotalAmount want 1, got %v", lvl.TotalAmount)
	}
	if ob.bidLevels.Len() != 1 {
		t.Fatalf("bid heap length want 1, got %d", ob.bidLevels.Len())
	}
	if _, ok := ob.Index["1"]; !ok {
		t.Fatalf("order index not recorded")
	}

	removed := ob.RemoveOrder("1")
	if removed == nil || removed.OrderID != "1" || removed.Price != testPrice(100) || removed.Amount != testQuantity(1) {
		t.Fatalf("RemoveOrder returned unexpected snapshot: %#v", removed)
	}

	if _, ok := ob.Bids[testPrice(100)]; ok {
		t.Fatalf("price level should be removed after last order")
	}
	if _, ok := ob.Index["1"]; ok {
		t.Fatalf("order index should be removed")
	}
	if ob.bidLevels.Len() != 0 {
		t.Fatalf("bid heap length want 0, got %d", ob.bidLevels.Len())
	}

	order.Amount = 9
	if removed.Amount != testQuantity(1) {
		t.Fatalf("removed snapshot must not track later mutations: got %v", removed.Amount)
	}
}

func TestRemoveOrderReturnsNilWhenOrderDoesNotExist(t *testing.T) {
	ob := NewOrderBook("BTC-USD")
	if removed := ob.RemoveOrder("missing"); removed != nil {
		t.Fatalf("missing removal should return nil, got %#v", removed)
	}
}

func TestEditOrderAmountIncreaseMovesToBack(t *testing.T) {
	ob := NewOrderBook("BTC-USD")
	o1 := newOrder("1", models.Bid, 100, 1)
	o2 := newOrder("2", models.Bid, 100, 1)
	ob.AddOrder(o1)
	ob.AddOrder(o2)

	// "1" 의 주문 수량 증가
	newAmt := testQuantity(2.0)
	req := models.EditOrderRequest{OrderID: "1", Price: testPrice(100), Amount: &newAmt}
	result := ob.EditOrder(req)
	if result == nil {
		t.Fatal("EditOrder should report the successful amount increase")
	}
	if result.Before.Amount != testQuantity(1) || result.After.Amount != testQuantity(2) || result.RequiresRematch {
		t.Fatalf("unexpected edit result: %#v", result)
	}

	lvl, ok := ob.Bids[testPrice(100)]
	if !ok {
		t.Fatalf("price level missing after edit")
	}

	// 최종 수량 3개
	if lvl.TotalAmount != testQuantity(3) {
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

	req := models.EditOrderRequest{OrderID: "1", Price: testPrice(101)}
	result := ob.EditOrder(req)
	if result == nil || !result.RequiresRematch {
		t.Fatalf("price change should require rematch: %#v", result)
	}
	if result.Before.Price != testPrice(100) || result.After.Price != testPrice(101) {
		t.Fatalf("unexpected price transition: %#v", result)
	}
	updated := result.After
	ob.AddOrder(&updated)

	// 호가 변경 시 주문이 호가 간 이동을 하는가
	if _, ok := ob.Bids[testPrice(100)]; ok {
		t.Fatalf("old price level should be removed")
	}
	lvl, ok := ob.Bids[testPrice(101)]
	if !ok {
		t.Fatalf("new price level not created")
	}
	if lvl.TotalAmount != testQuantity(1) {
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
	newAmt := testQuantity(1.0)
	req := models.EditOrderRequest{OrderID: "1", Price: testPrice(100), Amount: &newAmt}
	result := ob.EditOrder(req)
	if result == nil {
		t.Fatal("EditOrder should report the successful amount decrease")
	}
	if result.Before.Amount != testQuantity(2) || result.After.Amount != testQuantity(1) || result.RequiresRematch {
		t.Fatalf("unexpected edit result: %#v", result)
	}

	lvl, ok := ob.Bids[testPrice(100)]
	if !ok {
		t.Fatalf("price level missing after edit")
	}
	if lvl.TotalAmount != testQuantity(2) {
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

func TestEditOrderReturnsNilWhenNothingChanges(t *testing.T) {
	ob := NewOrderBook("BTC-USD")
	ob.AddOrder(newOrder("1", models.Bid, 100, 1))

	if result := ob.EditOrder(models.EditOrderRequest{OrderID: "1", Price: testPrice(100)}); result != nil {
		t.Fatalf("no-op edit should return nil, got %#v", result)
	}
}

func TestSnapshotReturnsAllLevelsWhenDepthIsZero(t *testing.T) {
	ob := NewOrderBook("BTC-USD")
	ob.AddOrder(newOrder("bid-1", models.Bid, 100, 1))
	ob.AddOrder(newOrder("bid-2", models.Bid, 99, 2))
	ob.AddOrder(newOrder("ask-1", models.Ask, 101, 3))
	ob.AddOrder(newOrder("ask-2", models.Ask, 102, 4))

	snapshot := ob.Snapshot(0)

	if snapshot.Ticker != "BTC-USD" {
		t.Fatalf("Ticker want BTC-USD, got %s", snapshot.Ticker)
	}
	assertLevel(t, snapshot.Bids, 0, 100, 1, 1)
	assertLevel(t, snapshot.Bids, 1, 99, 2, 3)
	assertLevel(t, snapshot.Asks, 0, 101, 3, 3)
	assertLevel(t, snapshot.Asks, 1, 102, 4, 7)
}

func TestSnapshotLimitsDepthPerSide(t *testing.T) {
	ob := NewOrderBook("BTC-USD")
	ob.AddOrder(newOrder("bid-1", models.Bid, 100, 1))
	ob.AddOrder(newOrder("bid-2", models.Bid, 99, 2))
	ob.AddOrder(newOrder("ask-1", models.Ask, 101, 3))
	ob.AddOrder(newOrder("ask-2", models.Ask, 102, 4))

	snapshot := ob.Snapshot(1)

	if len(snapshot.Bids) != 1 {
		t.Fatalf("bid depth want 1, got %d", len(snapshot.Bids))
	}
	if len(snapshot.Asks) != 1 {
		t.Fatalf("ask depth want 1, got %d", len(snapshot.Asks))
	}
	assertLevel(t, snapshot.Bids, 0, 100, 1, 1)
	assertLevel(t, snapshot.Asks, 0, 101, 3, 3)
}

func TestSnapshotDoesNotMutatePriceLevelIndexes(t *testing.T) {
	ob := NewOrderBook("BTC-USD")
	ob.AddOrder(newOrder("bid-1", models.Bid, 100, 1))
	ob.AddOrder(newOrder("bid-2", models.Bid, 99, 2))
	ob.AddOrder(newOrder("ask-1", models.Ask, 101, 3))
	ob.AddOrder(newOrder("ask-2", models.Ask, 102, 4))

	before := map[float64]int{
		100: ob.Bids[testPrice(100)].Index,
		99:  ob.Bids[testPrice(99)].Index,
		101: ob.Asks[testPrice(101)].Index,
		102: ob.Asks[testPrice(102)].Index,
	}

	ob.Snapshot(0)

	after := map[float64]int{
		100: ob.Bids[testPrice(100)].Index,
		99:  ob.Bids[testPrice(99)].Index,
		101: ob.Asks[testPrice(101)].Index,
		102: ob.Asks[testPrice(102)].Index,
	}
	for price, want := range before {
		if after[price] != want {
			t.Fatalf("price level %.4f Index mutated: got %d want %d", price, after[price], want)
		}
	}
}

func assertLevel(t *testing.T, levels []OrderBookLevel, index int, price, amount, cumulative float64) {
	t.Helper()

	if len(levels) <= index {
		t.Fatalf("missing level at index %d: got %d levels", index, len(levels))
	}
	got := levels[index]
	if got.Price != testPrice(price) || got.Amount != testQuantity(amount) || got.CumulativeAmount != testQuantity(cumulative) {
		t.Fatalf("level[%d] got price=%v amount=%v cumulative=%v, want price=%v amount=%v cumulative=%v",
			index, got.Price, got.Amount, got.CumulativeAmount, price, amount, cumulative)
	}
}
