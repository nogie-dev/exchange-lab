# clob-trading
 
포트폴리오용 CLOB(Central Limit Order Book) 트레이딩 엔진.
핵심 아이디어는 **가격대별 FIFO 큐**를 `PriceLevel`로 래핑하고, 레벨을 **Min/Max 힙**에 올려 최우선 호가를 O(1)/O(log N)로 다루는 것.
 
## Architecture
 
<p align="center">
  <img src="./asset/architecture.png" alt="CLOB Trading Architecture" width="720"/>
</p>
 
요청이 API Gateway를 통해 들어오면 Router가 트레이딩 페어별로 분기하고, 각 BookWorker가 명령을 PostgreSQL journal에 먼저 기록한 뒤 자신의 오더북에서 순차 처리한다. 매칭 알고리즘(`Match`)은 오더북을 직접 참조하며, 페어별 싱글 스레드에서 동작한다.
 
- **BookWorker**: 페어당 1개. OrderBook + Index + Match를 소유
- **유저 인증/잔고 검증**: 상위 User Service에서 처리한다고 가정 (점선 표시)
- **영속 journal + replay**: ticker sequence 순서로 명령을 재생하고 누락 체결 로그를 멱등적으로 복구
 
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
│   ├── api/             # 내부 command/query HTTP API
│   ├── engine/          # OrderBook, worker, Match 알고리즘
│   ├── journal/         # 영속 주문 명령과 replay 경계
│   ├── matchlog/        # 원시 체결 로그와 PostgreSQL 저장 경계
│   └── models/          # 주문 요청과 도메인 모델
├── db/                  # PostgreSQL migration과 sqlc query
└── til/                 # 학습 메모
```
 
## 실행
 
```bash
export MATCHING_ENGINE_DATABASE_URL='postgres://user:password@localhost:5432/matching'
go run ./cmd/server
```

기본 서버는 하나의 PostgreSQL connection pool을 공유하며, 체결 로그 transaction commit ACK를 받은 뒤 다음 주문을 처리한다. DB URL이 없거나 연결할 수 없으면 시작하지 않는다. 저장 실패 시 엔진은 자동 복구 없이 `HALTED` 상태가 되고, 신규 주문 명령과 `GET /ready`가 `503 Service Unavailable`을 반환한다.

create/amend/cancel 요청은 upstream에서 재시도에도 유지되는 `command_id`를 반드시 포함해야 한다. 서버는 journal append와 필요한 체결 로그 commit이 끝난 뒤에만 명령 성공을 반환한다. 재시작 시 journal replay와 로그 정합성 복구가 완료되기 전에는 HTTP 서버를 열지 않는다.

## Local Docker experiment

Docker Compose로 PostgreSQL과 기본 `BTC-USD` 엔진을 함께 실행할 수 있다.

```bash
docker compose up --build
curl http://127.0.0.1:8080/ready
```

엔진 컨테이너는 시작 전에 `db/migrations`를 Goose로 적용한다. PostgreSQL은 Compose 내부 네트워크에서만 열리며, 이 구성은 비밀번호 없는 로컬 실험용이다. 데이터베이스를 확인하려면 다음을 사용한다.

```bash
docker compose exec postgres psql -U matching -d matching
```

엔진이 준비된 뒤, 다음 Python 표준 라이브러리 스크립트는 매도 5,000개를 적재한 후 같은 가격의 매수 5,000개를 보내 총 1만 주문과 5,000 체결을 만든다.

```bash
python3 scripts/load_matched_orders.py
```

매도·매수 각각 1만 건이 필요하면 `--pairs 10000`을 지정한다.

```bash
python3 scripts/load_matched_orders.py --pairs 10000
```

매도 주문을 먼저 적재한 뒤 매수 주문 128개를 동시에 유지해 단일 ticker의 queue·journal·commit ACK 부하를 확인하려면 다음을 실행한다. 스크립트는 ask preload와 concurrent bid의 처리량, p50/p95/p99 HTTP 지연시간을 출력한다.

```bash
python3 scripts/load_matched_orders.py --concurrency 128
```

실험 데이터를 포함한 volume을 제거하려면 다음을 실행한다.

```bash
docker compose down -v
```
