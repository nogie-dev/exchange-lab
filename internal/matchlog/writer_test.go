package matchlog

import (
	"context"
	"errors"
	"testing"
)

type fakeStore struct {
	logs []MatchLog
	err  error
}

func (f *fakeStore) SaveMatchLog(ctx context.Context, log MatchLog) error {
	return f.SaveMatchLogs(ctx, []MatchLog{log})
}

func (f *fakeStore) SaveMatchLogs(_ context.Context, logs []MatchLog) error {
	f.logs = append(f.logs, logs...)
	return f.err
}

func TestWriterAcknowledgesStoredLogs(t *testing.T) {
	store := &fakeStore{}
	writer := NewWriter(store)
	in := make(chan PersistenceRequest, 1)
	request := NewPersistenceRequest([]MatchLog{{Ticker: "BTC-USD"}})
	in <- request
	close(in)

	writer.Run(context.Background(), in)
	if err := request.Wait(); err != nil {
		t.Fatalf("persistence acknowledgement returned error: %v", err)
	}

	if len(store.logs) != 1 {
		t.Fatalf("stored logs want 1, got %d", len(store.logs))
	}
	if store.logs[0].Ticker != "BTC-USD" {
		t.Fatalf("unexpected stored log: %#v", store.logs[0])
	}
}

func TestWriterAcknowledgesStoreFailure(t *testing.T) {
	storeErr := errors.New("database unavailable")
	writer := NewWriter(&fakeStore{err: storeErr})
	in := make(chan PersistenceRequest, 1)
	request := NewPersistenceRequest([]MatchLog{{Ticker: "BTC-USD"}})
	in <- request
	close(in)

	writer.Run(context.Background(), in)
	if err := request.Wait(); !errors.Is(err, storeErr) {
		t.Fatalf("persistence acknowledgement want %v, got %v", storeErr, err)
	}
}

func TestWriterRejectsMissingStore(t *testing.T) {
	writer := NewWriter(nil)
	in := make(chan PersistenceRequest, 1)
	request := NewPersistenceRequest([]MatchLog{{Ticker: "BTC-USD"}})
	in <- request
	close(in)

	writer.Run(context.Background(), in)
	if err := request.Wait(); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("persistence acknowledgement want ErrStoreUnavailable, got %v", err)
	}
}

func TestWriterAcknowledgesQueuedRequestAfterContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	writer := NewWriter(&fakeStore{})
	in := make(chan PersistenceRequest, 1)
	request := NewPersistenceRequest([]MatchLog{{Ticker: "BTC-USD"}})
	in <- request
	close(in)

	writer.Run(ctx, in)
	if err := request.Wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("persistence acknowledgement want context.Canceled, got %v", err)
	}
}
