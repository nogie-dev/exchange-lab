package models

import (
	"time"
)

type OrderType string
type Position string
type OrderStatus string

const (
	Limit  OrderType = "LIMIT"
	Market OrderType = "MARKET"

	Bid Position = "BID"
	Ask Position = "ASK"

	Pending  OrderStatus = "PENDING"  // 매칭 대기
	Filled   OrderStatus = "FILLED"   // 전체 체결
	Partial  OrderStatus = "PARTIAL"  // 부분 체결
	Canceled OrderStatus = "CANCELED" // 취소됨
)

type BookOrder struct {
	Ticker    string      `json:"ticker"`
	OrderID   string      `json:"order_id"`   // 주문 고유 ID
	UserID    string      `json:"user_id"`    // 사용자 ID
	OrderType OrderType   `json:"order_type"` // LIMIT, MARKET
	Position  Position    `json:"position"`   // BID, ASK
	Price     float64     `json:"price"`      // 가격
	Amount    float64     `json:"amount"`     // 수량
	Timestamp time.Time   `json:"timestamp"`  // 주문 생성 시간
	Status    OrderStatus `json:"status"`     // 현재 주문 상태
	Nonce     uint64      `json:"nonce"`
}
