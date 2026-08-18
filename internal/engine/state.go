package engine

import (
	"errors"
	"fmt"
	"sync"
)

var ErrEngineHalted = errors.New("engine halted")

// EngineState records the first fatal engine error and never resumes automatically.
type EngineState struct {
	mu    sync.RWMutex
	cause error
}

func NewEngineState() *EngineState {
	return &EngineState{}
}

func (s *EngineState) Halt(cause error) {
	if s == nil {
		return
	}
	if cause == nil {
		cause = ErrEngineHalted
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cause == nil {
		s.cause = cause
	}
}

func (s *EngineState) Err() error {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cause == nil {
		return nil
	}
	return haltedError(s.cause)
}

func haltedError(cause error) error {
	return fmt.Errorf("%w: %v", ErrEngineHalted, cause)
}
