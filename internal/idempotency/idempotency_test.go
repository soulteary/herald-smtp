package idempotency

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	for _, ttl := range []int{60, 0} {
		if store := NewStore(ttl, 0); store == nil {
			t.Fatalf("NewStore(%d) returned nil", ttl)
		}
	}
}

func TestStore_BeginFinishAndHit(t *testing.T) {
	store := NewStore(300, 10000)
	value, decision, err := store.Begin(context.Background(), "key", "request-1")
	if err != nil || decision != Proceed {
		t.Fatalf("Begin() = (%#v, %v, %v), want Proceed", value, decision, err)
	}
	store.Finish("key", "request-1", true, "msg-123")

	value, decision, err = store.Begin(context.Background(), "key", "request-1")
	if err != nil || decision != Hit {
		t.Fatalf("Begin() = (%#v, %v, %v), want Hit", value, decision, err)
	}
	if !value.OK || value.MessageID != "msg-123" {
		t.Errorf("cached value = %#v, want successful msg-123", value)
	}
}

func TestStore_FailedReservationCanRetry(t *testing.T) {
	store := NewStore(300, 10000)
	if _, decision, err := store.Begin(context.Background(), "key", "request-1"); err != nil || decision != Proceed {
		t.Fatalf("first Begin() decision = %v, error = %v", decision, err)
	}
	store.Finish("key", "request-1", false, "")
	if _, decision, err := store.Begin(context.Background(), "key", "request-1"); err != nil || decision != Proceed {
		t.Fatalf("retry Begin() decision = %v, error = %v, want Proceed", decision, err)
	}
	store.Finish("key", "request-1", false, "")
}

func TestStore_CapacityBoundsNewKeys(t *testing.T) {
	store := NewStore(300, 1)
	if _, decision, err := store.Begin(context.Background(), "first", "request-1"); err != nil || decision != Proceed {
		t.Fatalf("first Begin() = (%v, %v), want Proceed", decision, err)
	}
	if _, decision, err := store.Begin(context.Background(), "second", "request-2"); !errors.Is(err, ErrCapacityExceeded) || decision != Proceed {
		t.Fatalf("second Begin() = (%v, %v), want capacity error", decision, err)
	}
	store.Finish("first", "request-1", true, "msg-1")
	value, decision, err := store.Begin(context.Background(), "first", "request-1")
	if err != nil || decision != Hit || value.MessageID != "msg-1" {
		t.Fatalf("existing key at capacity = (%#v, %v, %v), want hit", value, decision, err)
	}
}

func TestStore_FailedReservationFreesCapacity(t *testing.T) {
	store := NewStore(300, 1)
	if _, decision, err := store.Begin(context.Background(), "first", "request-1"); err != nil || decision != Proceed {
		t.Fatalf("first Begin() = (%v, %v), want Proceed", decision, err)
	}
	store.Finish("first", "request-1", false, "")
	if _, decision, err := store.Begin(context.Background(), "second", "request-2"); err != nil || decision != Proceed {
		t.Fatalf("second Begin() = (%v, %v), want Proceed after release", decision, err)
	}
	store.Finish("second", "request-2", false, "")
}

func TestStore_ExpiredResultFreesCapacityImmediately(t *testing.T) {
	store := NewStore(300, 1)
	if _, decision, err := store.Begin(context.Background(), "first", "request-1"); err != nil || decision != Proceed {
		t.Fatalf("first Begin() = (%v, %v), want Proceed", decision, err)
	}
	store.Finish("first", "request-1", true, "msg-1")
	entry := store.m["first"]
	entry.expiresAt = time.Now().Add(-time.Second)
	store.m["first"] = entry
	store.nextCleanup = time.Now().Add(time.Minute)

	if _, decision, err := store.Begin(context.Background(), "second", "request-2"); err != nil || decision != Proceed {
		t.Fatalf("second Begin() = (%v, %v), want Proceed after expired entry cleanup", decision, err)
	}
	store.Finish("second", "request-2", false, "")
}

func TestStore_Conflict(t *testing.T) {
	store := NewStore(300, 10000)
	if _, decision, err := store.Begin(context.Background(), "key", "request-1"); err != nil || decision != Proceed {
		t.Fatalf("first Begin() decision = %v, error = %v", decision, err)
	}
	if _, decision, err := store.Begin(context.Background(), "key", "request-2"); err != nil || decision != Conflict {
		t.Fatalf("conflicting Begin() decision = %v, error = %v, want Conflict", decision, err)
	}
	store.Finish("key", "request-1", false, "")
}

func TestStore_ConcurrentDuplicateWaitsForOwner(t *testing.T) {
	store := NewStore(300, 10000)
	if _, decision, err := store.Begin(context.Background(), "key", "request-1"); err != nil || decision != Proceed {
		t.Fatalf("owner Begin() decision = %v, error = %v", decision, err)
	}

	type beginResult struct {
		value    cached
		decision Decision
		err      error
	}
	resultCh := make(chan beginResult, 1)
	go func() {
		value, decision, err := store.Begin(context.Background(), "key", "request-1")
		resultCh <- beginResult{value: value, decision: decision, err: err}
	}()
	select {
	case result := <-resultCh:
		t.Fatalf("duplicate returned before owner finished: %#v", result)
	case <-time.After(25 * time.Millisecond):
	}

	store.Finish("key", "request-1", true, "msg-123")
	select {
	case result := <-resultCh:
		if result.err != nil || result.decision != Hit || result.value.MessageID != "msg-123" {
			t.Fatalf("duplicate result = %#v, want cached hit", result)
		}
	case <-time.After(time.Second):
		t.Fatal("duplicate did not resume after owner finished")
	}
}

func TestStore_WaitHonorsContextCancellation(t *testing.T) {
	store := NewStore(300, 10000)
	if _, decision, err := store.Begin(context.Background(), "key", "request-1"); err != nil || decision != Proceed {
		t.Fatalf("owner Begin() decision = %v, error = %v", decision, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := store.Begin(ctx, "key", "request-1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting Begin() error = %v, want context.Canceled", err)
	}
	store.Finish("key", "request-1", false, "")
}

func TestStore_ExpiredEntryCanBeReused(t *testing.T) {
	store := NewStore(300, 10000)
	if _, decision, err := store.Begin(context.Background(), "key", "request-1"); err != nil || decision != Proceed {
		t.Fatalf("Begin() decision = %v, error = %v", decision, err)
	}
	store.Finish("key", "request-1", true, "msg-old")
	entry := store.m["key"]
	entry.expiresAt = time.Now().Add(-time.Second)
	store.m["key"] = entry

	if _, decision, err := store.Begin(context.Background(), "key", "request-2"); err != nil || decision != Proceed {
		t.Fatalf("expired Begin() decision = %v, error = %v, want Proceed", decision, err)
	}
	store.Finish("key", "request-2", false, "")
}

func TestStore_BeginCleansExpiredEntries(t *testing.T) {
	store := NewStore(300, 10000)
	store.m["expired"] = entry{complete: true, expiresAt: time.Now().Add(-time.Second)}
	store.nextCleanup = time.Time{}
	if _, decision, err := store.Begin(context.Background(), "new", "request"); err != nil || decision != Proceed {
		t.Fatalf("Begin() decision = %v, error = %v", decision, err)
	}
	if _, exists := store.m["expired"]; exists {
		t.Fatal("Begin() should remove expired entries during scheduled cleanup")
	}
	store.Finish("new", "request", false, "")
}

func TestStore_FinishIgnoresUnknownOrMismatchedReservation(t *testing.T) {
	store := NewStore(300, 10000)
	store.Finish("missing", "request", true, "msg")
	if _, decision, err := store.Begin(context.Background(), "key", "request-1"); err != nil || decision != Proceed {
		t.Fatalf("Begin() decision = %v, error = %v", decision, err)
	}
	store.Finish("key", "request-2", true, "msg")
	store.Finish("key", "request-1", true, "msg")
	store.Finish("key", "request-1", true, "other")
	value, decision, err := store.Begin(context.Background(), "key", "request-1")
	if err != nil || decision != Hit || value.MessageID != "msg" {
		t.Fatalf("cached result = (%#v, %v, %v), want original hit", value, decision, err)
	}
}
