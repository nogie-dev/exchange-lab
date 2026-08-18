package matchlog

import (
	"context"
	"errors"
	"log/slog"
)

var ErrStoreUnavailable = errors.New("match log store unavailable")

type PersistenceRequest struct {
	Logs   []MatchLog
	result chan error
}

func NewPersistenceRequest(logs []MatchLog) PersistenceRequest {
	return PersistenceRequest{
		Logs:   logs,
		result: make(chan error, 1),
	}
}

func (r PersistenceRequest) Acknowledge(err error) {
	if r.result != nil {
		r.result <- err
	}
}

func (r PersistenceRequest) Wait() error {
	if r.result == nil {
		return ErrStoreUnavailable
	}
	return <-r.result
}

type Writer struct {
	store Store
}

func NewWriter(store Store) *Writer {
	return &Writer{store: store}
}

func (w *Writer) Run(ctx context.Context, in <-chan PersistenceRequest) {
	for request := range in {
		var err error
		switch {
		case len(request.Logs) == 0:
		case ctx.Err() != nil:
			err = ctx.Err()
		case w.store == nil:
			err = ErrStoreUnavailable
		default:
			err = w.store.SaveMatchLogs(ctx, request.Logs)
		}
		if err != nil {
			slog.Error("match log store failed", "error", err)
		}
		request.Acknowledge(err)
	}
}
