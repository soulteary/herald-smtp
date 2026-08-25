package idempotency

import (
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	s := NewStore(60)
	if s == nil {
		t.Fatal("NewStore(60) returned nil")
	}
	s0 := NewStore(0)
	if s0 == nil {
		t.Fatal("NewStore(0) returned nil")
	}
}

func TestStore_SetAndGetHit(t *testing.T) {
	s := NewStore(300)
	key := "idem-key-1"
	s.Set(key, "request-1", true, "msg-123")
	c, hit, conflict := s.Get(key, "request-1")
	if !hit {
		t.Fatal("expected hit after Set")
	}
	if conflict {
		t.Fatal("matching request should not conflict")
	}
	if !c.OK || c.MessageID != "msg-123" {
		t.Errorf("got OK=%v MessageID=%q, want OK=true MessageID=msg-123", c.OK, c.MessageID)
	}
}

func TestStore_GetMissNoSet(t *testing.T) {
	s := NewStore(300)
	_, hit, conflict := s.Get("nonexistent", "request")
	if hit {
		t.Fatal("expected miss for key never set")
	}
	if conflict {
		t.Fatal("missing key should not conflict")
	}
}

func TestStore_GetMissAfterExpiry(t *testing.T) {
	s := NewStore(1) // 1 second TTL
	key := "expire-key"
	s.Set(key, "request-1", true, "msg-456")
	if _, hit, _ := s.Get(key, "request-1"); !hit {
		t.Fatal("expected hit immediately after Set")
	}
	time.Sleep(2 * time.Second)
	_, hit, conflict := s.Get(key, "request-1")
	if hit {
		t.Fatal("expected miss after TTL expiry")
	}
	if conflict {
		t.Fatal("expired key should not conflict")
	}
	if _, exists := s.m[key]; exists {
		t.Fatal("expired key should be removed")
	}
}

func TestStore_SetFailureThenGet(t *testing.T) {
	s := NewStore(300)
	key := "fail-key"
	s.Set(key, "request-1", false, "")
	c, hit, conflict := s.Get(key, "request-1")
	if !hit {
		t.Fatal("expected hit for cached failure")
	}
	if conflict {
		t.Fatal("matching failed request should not conflict")
	}
	if c.OK || c.MessageID != "" {
		t.Errorf("got OK=%v MessageID=%q, want OK=false MessageID=", c.OK, c.MessageID)
	}
}

func TestStore_GetConflict(t *testing.T) {
	s := NewStore(300)
	s.Set("same-key", "request-1", true, "msg-123")
	if _, hit, conflict := s.Get("same-key", "request-2"); hit || !conflict {
		t.Fatalf("Get() hit=%v conflict=%v, want false, true", hit, conflict)
	}
}

func TestStore_SetCleansExpiredEntries(t *testing.T) {
	s := NewStore(300)
	s.m["expired"] = entry{expiresAt: time.Now().Add(-time.Second)}
	s.nextCleanup = time.Time{}
	s.Set("new", "request", true, "msg-new")
	if _, exists := s.m["expired"]; exists {
		t.Fatal("Set() should remove expired entries during scheduled cleanup")
	}
	if _, exists := s.m["new"]; !exists {
		t.Fatal("Set() should retain the new entry")
	}
}
