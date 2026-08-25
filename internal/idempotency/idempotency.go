package idempotency

import (
	"sync"
	"time"
)

type entry struct {
	fingerprint string
	ok          bool
	messageID   string
	expiresAt   time.Time
}

// Store is an in-memory idempotency store. The same key and request fingerprint
// within the TTL returns a cached response; mismatched fingerprints conflict.
type Store struct {
	mu              sync.RWMutex
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

// Get returns a cached result for an unexpired matching request. conflict is
// true when the same key was already used for different request content.
func (s *Store) Get(key, fingerprint string) (cached, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[key]
	if !ok {
		return cached{}, false, false
	}
	if time.Now().After(e.expiresAt) {
		delete(s.m, key)
		return cached{}, false, false
	}
	if e.fingerprint != fingerprint {
		return cached{}, false, true
	}
	return cached{OK: e.ok, MessageID: e.messageID}, true, false
}

// Set stores the result for key with TTL.
func (s *Store) Set(key, fingerprint string, ok bool, messageID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if !now.Before(s.nextCleanup) {
		for storedKey, storedEntry := range s.m {
			if now.After(storedEntry.expiresAt) {
				delete(s.m, storedKey)
			}
		}
		s.nextCleanup = now.Add(s.cleanupInterval)
	}
	s.m[key] = entry{
		fingerprint: fingerprint,
		ok:          ok,
		messageID:   messageID,
		expiresAt:   now.Add(time.Duration(s.ttlSec) * time.Second),
	}
}
