package ratelimit

import (
	"testing"
	"time"
)

func TestUnderLimitAllowed(t *testing.T) {
	l := NewLimiter(5)
	for i := 0; i < 5; i++ {
		if !l.Allow("a") {
			t.Fatalf("request %d should be allowed", i)
		}
	}
}

func TestOverLimitBlocked(t *testing.T) {
	l := NewLimiter(2)
	if !l.Allow("a") {
		t.Fatal("first should be allowed")
	}
	if !l.Allow("a") {
		t.Fatal("second should be allowed")
	}
	if l.Allow("a") {
		t.Fatal("third should be blocked")
	}
}

func TestDifferentAgentsIndependent(t *testing.T) {
	l := NewLimiter(1)
	if !l.Allow("a") {
		t.Fatal("a first should be allowed")
	}
	if !l.Allow("b") {
		t.Fatal("b first should be allowed")
	}
	if l.Allow("a") {
		t.Fatal("a second should be blocked")
	}
}

func TestResetWorks(t *testing.T) {
	l := NewLimiter(1)
	if !l.Allow("a") {
		t.Fatal("first should be allowed")
	}
	if l.Allow("a") {
		t.Fatal("second should be blocked")
	}
	l.Reset("a")
	if !l.Allow("a") {
		t.Fatal("after reset should be allowed")
	}
}

func TestWindowExpiry(t *testing.T) {
	l := NewLimiter(1)
	l.window = 50 * time.Millisecond
	if !l.Allow("a") {
		t.Fatal("first should be allowed")
	}
	if l.Allow("a") {
		t.Fatal("second should be blocked")
	}
	time.Sleep(80 * time.Millisecond)
	if !l.Allow("a") {
		t.Fatal("after window expiry should be allowed")
	}
}

func TestAutoCleanup(t *testing.T) {
	l := NewLimiter(1)
	l.window = 50 * time.Millisecond
	l.Allow("a")
	time.Sleep(80 * time.Millisecond)
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.entries["a"]; ok {
		t.Fatal("stale entry should be cleaned up")
	}
}
