package orderevent

import (
	"context"
	"errors"
	"testing"
)

type writerTestStore struct {
	batches [][]Event
	err     error
}

func (s *writerTestStore) SaveEvent(ctx context.Context, event Event) error {
	return s.SaveEvents(ctx, []Event{event})
}

func (s *writerTestStore) SaveEvents(_ context.Context, events []Event) error {
	batch := append([]Event(nil), events...)
	s.batches = append(s.batches, batch)
	return s.err
}

func TestWriterAcknowledgesBatchesInQueueOrder(t *testing.T) {
	store := &writerTestStore{}
	writer := NewWriter(store)
	in := make(chan PersistenceRequest, 2)
	first := NewPersistenceRequest([]Event{{EventID: "event-1"}, {EventID: "event-2"}})
	second := NewPersistenceRequest([]Event{{EventID: "event-3"}})
	in <- first
	in <- second
	close(in)

	writer.Run(context.Background(), in)
	if err := first.Wait(); err != nil {
		t.Fatalf("first persistence acknowledgement returned error: %v", err)
	}
	if err := second.Wait(); err != nil {
		t.Fatalf("second persistence acknowledgement returned error: %v", err)
	}

	if len(store.batches) != 2 {
		t.Fatalf("stored batches want 2, got %d", len(store.batches))
	}
	if len(store.batches[0]) != 2 || store.batches[0][0].EventID != "event-1" || store.batches[0][1].EventID != "event-2" {
		t.Fatalf("unexpected first stored batch: %#v", store.batches[0])
	}
	if len(store.batches[1]) != 1 || store.batches[1][0].EventID != "event-3" {
		t.Fatalf("unexpected second stored batch: %#v", store.batches[1])
	}
}

func TestWriterAcknowledgesStoreFailure(t *testing.T) {
	storeErr := errors.New("database unavailable")
	writer := NewWriter(&writerTestStore{err: storeErr})
	in := make(chan PersistenceRequest, 1)
	request := NewPersistenceRequest([]Event{{EventID: "event-1"}})
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
	request := NewPersistenceRequest([]Event{{EventID: "event-1"}})
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
	store := &writerTestStore{}
	writer := NewWriter(store)
	in := make(chan PersistenceRequest, 1)
	request := NewPersistenceRequest([]Event{{EventID: "event-1"}})
	in <- request
	close(in)

	writer.Run(ctx, in)
	if err := request.Wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("persistence acknowledgement want context.Canceled, got %v", err)
	}
	if len(store.batches) != 0 {
		t.Fatalf("canceled writer must not call the store, got %#v", store.batches)
	}
}

func TestWriterAcknowledgesEmptyBatchWithoutStoreCall(t *testing.T) {
	store := &writerTestStore{}
	writer := NewWriter(store)
	in := make(chan PersistenceRequest, 1)
	request := NewPersistenceRequest(nil)
	in <- request
	close(in)

	writer.Run(context.Background(), in)
	if err := request.Wait(); err != nil {
		t.Fatalf("empty persistence acknowledgement returned error: %v", err)
	}
	if len(store.batches) != 0 {
		t.Fatalf("empty batch must not call the store, got %#v", store.batches)
	}
}

func TestZeroPersistenceRequestReportsUnavailable(t *testing.T) {
	var request PersistenceRequest
	if err := request.Wait(); !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("zero request want ErrStoreUnavailable, got %v", err)
	}
}
