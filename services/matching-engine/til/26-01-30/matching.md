# Matching 설계 메모

  - **목표**: 학습용 CLOB에서 라우팅·매칭·오더북 분리 방식을 정리.

  ## 현재 구조 인식
  - `OrderBook`은 `Add/Remove/Edit`로 상태를 조작하고, 매칭 루프는 아직 없음.
  - `Matcher`는 `books map[ticker]*OrderBook`만 있고 실사용되지 않음(`GetOrderBook`은 fatal).
  - `main`은 `NewOrderBook`을 직접 만들고 `AddOrder`만 호출하는 단순 흐름.

  ## 라우팅/워커 방향
  - **Router**: `OrderRouter(ev)`가 `ev.Req.Ticker`로 책 워커 채널에 이벤트를 전달. 없으면 워커/오더북을 생성.
  - **BookWorker**: 티커별 단일 고루틴. 이벤트(New/Cancel/Edit)를 처리해 `OrderBook` 상태를 바꾸고, 이벤트마
  다 교차가 없어질 때까지 매칭 실행.
  - **Matcher 역할**: “매칭 로직”을 수행하는 컴포넌트로 한정. 오더북 레지스트리는 라우터/워커 쪽에서 관리하고, Matcher는 주어진 오더북에서 교차 여부 판단 → 필/파트
    필/잔여량 계산 → 체결 이벤트 생성만 담당하도록 역할 최소화

  ## 매칭 트리거와 범위
  - 트리거: 이벤트 처리 직후(새 주문, 가격/수량 변경, 취소 이후) 매칭을 돌린다. 별도 폴링 루프는 불필요.
  - 범위: `bestBid >= bestAsk`일 동안 반복. 각 사이드 최상위 레벨만 비교/소진하며 교차 해소.

