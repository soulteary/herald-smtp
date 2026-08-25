package idempotency

import (
	"context"
	"sync"
	"time"
)

type entry struct {
	fingerprint string
	ready       chan struct{}
	complete    bool
	ok          bool
	messageID   string
	expiresAt   time.Time
}

// Store is an in-memory idempotency store. The same key and request fingerprint
// within the TTL returns a cached response; mismatched fingerprints conflict.
type Store struct {
	mu              sync.Mutex
	m               map[string]entry
	ttlSec          int
	cleanupInterval time.Duration
	nextCleanup     time.Time
}

// NewStore creates a store with the given TTL in seconds.
func NewStore(ttlSec int) *Store {
	s := &Store{m: make(map[string]entry), ttlSec: ttlSec}
	if s.ttlSec <= 0 {
		s.ttlSec = 300
	}
	s.cleanupInterval = time.Duration(s.ttlSec) * time.Second
	if s.cleanupInterval > time.Minute {
		s.cleanupInterval = time.Minute
	}
	s.nextCleanup = time.Now().Add(s.cleanupInterval)
	return s
}

type cached struct {
	OK        bool
	MessageID string
}

// Decision describes how a caller should handle an idempotency lookup.
type Decision uint8

const (
	// Proceed means the caller owns a new in-flight reservation and must Finish it.
	Proceed Decision = iota
	// Hit means a completed matching request was found.
	Hit
	// Conflict means the key is already associated with different request content.
	Conflict
)

// Begin reserves a new key or waits for an identical in-flight request. This
// closes the lookup/send/store race that could otherwise send the same email
// more than once when duplicate requests arrive concurrently.
func (s *Store) Begin(ctx context.Context, key, fingerprint string) (cached, Decision, error) {
	for {
		now := time.Now()
		s.mu.Lock()
		s.cleanupExpired(now)
		e, ok := s.m[key]
		if !ok {
			s.m[key] = entry{fingerprint: fingerprint, ready: make(chan struct{})}
			s.mu.Unlock()
			return cached{}, Proceed, nil
		}
		if e.complete && now.After(e.expiresAt) {
			delete(s.m, key)
			s.mu.Unlock()
			continue
		}
		if e.fingerprint != fingerprint {
			s.mu.Unlock()
			return cached{}, Conflict, nil
		}
		if e.complete {
			value := cached{OK: e.ok, MessageID: e.messageID}
			s.mu.Unlock()
			return value, Hit, nil
		}
		ready := e.ready
		s.mu.Unlock()

		select {
		case <-ready:
			// The owner either completed or aborted. Re-check under the lock.
		case <-ctx.Done():
			return cached{}, Proceed, ctx.Err()
		}
	}
}

// Finish completes an owned reservation. Successful results remain cached;
// failed results are removed so a later request can retry.
func (s *Store) Finish(key, fingerprint string, ok bool, messageID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, exists := s.m[key]
	if !exists || e.complete || e.fingerprint != fingerprint {
		return
	}
	if !ok {
		delete(s.m, key)
		close(e.ready)
		return
	}
	e.complete = true
	e.ok = true
	e.messageID = messageID
	e.expiresAt = time.Now().Add(time.Duration(s.ttlSec) * time.Second)
	s.m[key] = e
	close(e.ready)
}

func (s *Store) cleanupExpired(now time.Time) {
	if now.Before(s.nextCleanup) {
		return
	}
	for storedKey, storedEntry := range s.m {
		if storedEntry.complete && now.After(storedEntry.expiresAt) {
			delete(s.m, storedKey)
		}
	}
	s.nextCleanup = now.Add(s.cleanupInterval)
}
