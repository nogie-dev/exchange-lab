# clob-trading
 
포트폴리오용 CLOB(Central Limit Order Book) 트레이딩 엔진.
핵심 아이디어는 **가격대별 FIFO 큐**를 `PriceLevel`로 래핑하고, 레벨을 **Min/Max 힙**에 올려 최우선 호가를 O(1)/O(log N)로 다루는 것.
 
## Architecture
 
<p align="center">
  <img src="./asset/architecture.png" alt="CLOB Trading Architecture" width="720"/>
</p>
 
요청이 API Gateway를 통해 들어오면 Router가 트레이딩 페어별로 분기하고, 각 BookWorker가 자신의 오더북에서 독립적으로 주문을 처리한다. 매칭 알고리즘(`Match`)은 오더북을 직접 참조하는 순수 함수로, 페어별 싱글 스레드에서 동작한다.
 
- **BookWorker**: 페어당 1개. OrderBook + Index + Match를 소유
- **유저 인증/잔고 검증**: 상위 User Service에서 처리한다고 가정 (점선 표시)
- **WAL + Snapshot**: 체결 이벤트를 기록하고, 리부팅 시 리플레이하여 오더북 상태를 복원
 
## 데이터 구조
 
| 구조 | 역할 | 시간복잡도 |
|---|---|---|
| `Queue` (container/list) | 같은 가격대 주문을 FIFO로 유지 | Push/Remove O(1) |
| `PriceLevel` | `{Price, Queue, TotalAmount, Index}`로 큐를 래핑, 힙 내 위치 저장 | - |
| `MaxPriceHeap` / `MinPriceHeap` | Bid/Ask 가격 우선순위 유지 | Peek O(1), Push/Pop O(log N) |
| `OrderBook.Index` | `orderID → *list.Element` 매핑 | 조회/삭제 O(1) |
 
## 주문 라이프사이클
 
### 생성
`CreateOrder` → `AddOrder`가 가격대 없으면 새 `PriceLevel`을 만들고 힙에 삽입한 뒤 큐에 푸시, 인덱스에 기록.
 
### 매칭
`Match(book, order)`가 incoming 주문을 받아 반대쪽 최우선 호가와 비교하며 price-time priority로 체결. 잔량이 남으면 오더북에 resting order로 등록.
 
```
Bid 진입 → bestAsk와 비교 → price 교차 시 tradeAmt = min(incoming, target) → 체결
                           → 교차 안하면 break → 잔량을 Bid 오더북에 등록
```
 
### 취소
`RemoveOrder`가 인덱스로 O(1) 접근하여 큐에서 제거, `TotalAmount` 차감. 레벨 큐가 비면 힙·맵에서 가격대 삭제.
 
### 수정
`EditOrder(EditOrderRequest)`로 처리.
- **가격 변경**: 기존 레벨에서 제거 후 새 가격으로 재삽입 (순번 리셋)
- **수량 증가**: 공정성을 위해 기존 레벨에서 제거 후 재삽입 (뒤로 배치)
- **수량 감소**: 위치 유지, 수량·누적만 조정
 
## 프로젝트 구조
 
```
clob-trading/
├── cmd/server/          # 서버 엔트리포인트
├── internal/
│   ├── engine/          # OrderBook, Heap, Match 알고리즘
│   └── models/          # BookOrder, PriceLevel 등 도메인 모델
└── til/                 # 학습 메모
```
 
## 실행
 
```bash
go run cmd/server/main.go
```
