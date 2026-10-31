package rooms

import (
	"fmt"
	"sync"
	"time"
)

type LoopGuard struct {
	maxPerPair int
	window     time.Duration

	mu      sync.Mutex
	entries map[string][]time.Time
}

func NewLoopGuard(maxPerPair int, windowMinutes int) *LoopGuard {
	window := time.Duration(windowMinutes) * time.Minute
	if window <= 0 {
		window = 10 * time.Millisecond
	}
	return &LoopGuard{
		maxPerPair: maxPerPair,
		window:     window,
		entries:    make(map[string][]time.Time),
	}
}

func (g *LoopGuard) key(senderID, recipientID string) string {
	return senderID + "\x00" + recipientID
}

func (g *LoopGuard) prune(key string, now time.Time) {
	cutoff := now.Add(-g.window)
	var kept []time.Time
	for _, t := range g.entries[key] {
		if !t.Before(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(g.entries, key)
	} else {
		g.entries[key] = kept
	}
}

func (g *LoopGuard) Check(senderID, recipientID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	key := g.key(senderID, recipientID)
	now := time.Now()
	g.prune(key, now)

	if len(g.entries[key]) >= g.maxPerPair {
		return fmt.Errorf("loop guard: message limit exceeded for %s -> %s", senderID, recipientID)
	}
	return nil
}

func (g *LoopGuard) Record(senderID, recipientID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	key := g.key(senderID, recipientID)
	now := time.Now()
	g.prune(key, now)
	g.entries[key] = append(g.entries[key], now)
}
