package ratelimit

import (
	"sync"
	"time"
)

// Limiter implements a per-key sliding-window rate limiter.
type Limiter struct {
	rpm    int
	window time.Duration

	mu      sync.Mutex
	entries map[string][]time.Time
}

// NewLimiter creates a limiter allowing rpm requests per minute per key.
func NewLimiter(rpm int) *Limiter {
	if rpm <= 0 {
		rpm = 60
	}
	l := &Limiter{
		rpm:     rpm,
		window:  time.Minute,
		entries: make(map[string][]time.Time),
	}
	go l.cleanupLoop()
	return l
}

// Allow reports whether a request for agentID is permitted under the limit.
func (l *Limiter) Allow(agentID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.cleanupLocked(now)

	cutoff := now.Add(-l.window)
	var kept []time.Time
	for _, t := range l.entries[agentID] {
		if !t.Before(cutoff) {
			kept = append(kept, t)
		}
	}

	if len(kept) >= l.rpm {
		l.entries[agentID] = kept
		return false
	}

	kept = append(kept, now)
	l.entries[agentID] = kept
	return true
}

// Reset clears the recorded requests for agentID.
func (l *Limiter) Reset(agentID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, agentID)
}

// cleanupLoop periodically removes stale entries.
func (l *Limiter) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		l.cleanupLocked(time.Now())
		l.mu.Unlock()
	}
}

// cleanupLocked removes stale entries. Caller must hold l.mu.
func (l *Limiter) cleanupLocked(now time.Time) {
	cutoff := now.Add(-l.window)
	for key, ts := range l.entries {
		var kept []time.Time
		for _, t := range ts {
			if !t.Before(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(l.entries, key)
		} else {
			l.entries[key] = kept
		}
	}
}
