package engine

import (
	"fmt"
	"sync"
)

// Router holds book-specific workers to dispatch incoming events/orders.
type Router struct {
	mu      sync.RWMutex
	workers map[string]*BookWorker
}

// NewRouter initializes an empty router with no workers.
func NewRouter() *Router {
	return &Router{
		workers: make(map[string]*BookWorker),
	}
}

// Register adds a pre-created worker for a ticker.
// Use at startup after restoring orderbooks from storage.
func (r *Router) Register(ticker string, w *BookWorker) error {
	if ticker == "" {
		return fmt.Errorf("empty ticker")
	}
	if w == nil {
		return fmt.Errorf("nil worker for ticker %s", ticker)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workers[ticker] = w
	return nil
}

func (r *Router) OrderRouter(ev Event) error {
	ticker := ev.Ticker
	if ticker == "" {
		return fmt.Errorf("empty ticker")
	}

	r.mu.RLock()
	w := r.workers[ticker]
	r.mu.RUnlock()

	if w == nil {
		return fmt.Errorf("unknown ticker: %s", ticker)
	}

	w.in <- ev
	return nil
}
