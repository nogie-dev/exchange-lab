package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrEmptyTicker   = errors.New("empty ticker")
	ErrUnknownTicker = errors.New("unknown ticker")
	ErrTickerExists  = errors.New("ticker already registered")
	ErrRouterClosed  = errors.New("router closed")
)

// Router holds book-specific workers to dispatch incoming events/orders.
type Router struct {
	mu      sync.RWMutex
	workers map[string]*BookWorker
	state   *EngineState
	closed  bool
}

// NewRouter initializes an empty router with no workers.
func NewRouter() *Router {
	return NewRouterWithState(NewEngineState())
}

func NewRouterWithState(state *EngineState) *Router {
	if state == nil {
		state = NewEngineState()
	}
	return &Router{
		workers: make(map[string]*BookWorker),
		state:   state,
	}
}

// Register adds a pre-created worker for a ticker.
// Use at startup after restoring orderbooks from storage.
func (r *Router) Register(ticker string, w *BookWorker) error {
	if ticker == "" {
		return ErrEmptyTicker
	}
	if w == nil {
		return fmt.Errorf("nil worker for ticker %s", ticker)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrRouterClosed
	}
	if _, exists := r.workers[ticker]; exists {
		return fmt.Errorf("%w: %s", ErrTickerExists, ticker)
	}
	w.state = r.state
	r.workers[ticker] = w
	return nil
}

// HasTicker reports whether a worker is registered for the ticker.
func (r *Router) HasTicker(ticker string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.workers[ticker]
	return ok
}

func (r *Router) OrderRouter(ev Event) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return ErrRouterClosed
	}
	w, err := r.workerLocked(ev.Ticker)
	if err != nil {
		return err
	}
	if err := r.state.Err(); err != nil {
		return err
	}

	result := make(chan error, 1)
	ev.commandResult = result
	w.in <- ev
	return <-result
}

// OrderBookSnapshot reads a snapshot on the owning worker's event queue.
func (r *Router) OrderBookSnapshot(ticker string, depth int) (OrderBookSnapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return OrderBookSnapshot{}, ErrRouterClosed
	}
	w, err := r.workerLocked(ticker)
	if err != nil {
		return OrderBookSnapshot{}, err
	}

	result := make(chan OrderBookSnapshot, 1)
	w.in <- Event{
		Type:           snapshotOrderBook,
		Ticker:         ticker,
		snapshotDepth:  depth,
		snapshotResult: result,
	}
	return <-result, nil
}

func (r *Router) Ready() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return ErrRouterClosed
	}
	return r.state.Err()
}

func (r *Router) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	workers := make(map[*BookWorker]struct{}, len(r.workers))
	for _, worker := range r.workers {
		workers[worker] = struct{}{}
	}
	if !r.closed {
		r.closed = true
		for worker := range workers {
			close(worker.in)
		}
	}
	r.mu.Unlock()

	for worker := range workers {
		select {
		case <-worker.Done():
		case <-ctx.Done():
			return fmt.Errorf("shutdown workers: %w", ctx.Err())
		}
	}
	return nil
}

func (r *Router) workerLocked(ticker string) (*BookWorker, error) {
	if ticker == "" {
		return nil, ErrEmptyTicker
	}

	w := r.workers[ticker]
	if w == nil {
		return nil, fmt.Errorf("%w: %s", ErrUnknownTicker, ticker)
	}
	return w, nil
}
