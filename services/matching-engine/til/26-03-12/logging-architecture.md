# 로깅 아키텍처 고민

## 고민 지점
BookWorker에서 이벤트를 받아 처리할 때 주문 로그를 어디서 찍어야 하는가.

**Option A: bookworker.go의 switch ev.Type 안에서 찍기**
```go
case NewOrder:
    order := CreateOrder(*ev.NewOrder)
    slog.Info("order received", "orderID", order.OrderID, ...)
    residual := Match(w.OrderBook, &order)
    ...
```

**Option B: OrderBook 메서드(AddOrder 등) 내부에서 찍기**
```go
func (ob *OrderBook) AddOrder(order *models.BookOrder) {
    ...
    ob.Index[order.OrderID] = lvl.Queue.Push(order)
    slog.Info("order added", ...)
}
```

## 결론: 로그 단위 원칙

**(Claude) "비즈니스 의도가 있는 지점에서 찍어라. 구현 세부사항에서는 찍지 않는다."**
- 로그의 목적
    - 문제 원인 파악
    - 사용자 행동 추적
    - 비즈니스 흐름 확인
    - 장애 분석

### 로그를 찍어야 하는 지점
- `bookworker.go` switch 분기 → 각 이벤트 타입의 최상위 처리 지점
- Public 메서드 중 비즈니스 이벤트를 대표하는 것 (RemoveOrder 처리 완료 직후 등)

### 로그를 찍으면 안 되는 지점
- `removeElement` (private helper) → RemoveOrder/EditOrder에서 공용 호출, 컨텍스트 없음
- `AddOrder` 내부 → 세 곳에서 호출됨 (잔여분 등록 / 가격변경 재삽입 / 수량증가 재삽입), 로그만 봐서는 왜 추가됐는지 알 수 없음

### 각 지점별 로그 책임
| 지점 | 로그 위치 | 이유 |
|---|---|---|
| 신규 주문 접수 | bookworker.go NewOrder 분기 | 이벤트 수신 의도 |
| 체결 발생 | match.go 루프 내부 (tradeAmt 결정 직후) | 호가별 체결 건 각각 기록 필요 |
| 잔여분 북 등록 | bookworker.go, Match 직후 | "partial fill residual" 컨텍스트 |
| 주문 취소 | RemoveOrder 처리 완료 직후 | public 메서드, 비즈니스 이벤트 |
| 주문 수정 | EditOrder 각 case 분기 끝 | price_changed / amount_changed 구분 |

## 로거 선택
- `log/slog` 사용 (Go 1.21+ 내장, 의존성 없음)