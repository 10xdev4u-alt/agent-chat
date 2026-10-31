package rooms

import (
	"testing"
	"time"
)

func TestLoopGuardUnderLimit(t *testing.T) {
	lg := NewLoopGuard(2, 1)
	if err := lg.Check("a", "b"); err != nil {
		t.Fatal(err)
	}
	lg.Record("a", "b")
	if err := lg.Check("a", "b"); err != nil {
		t.Fatal(err)
	}
}

func TestLoopGuardOverLimit(t *testing.T) {
	lg := NewLoopGuard(2, 1)
	lg.Record("a", "b")
	lg.Record("a", "b")
	if err := lg.Check("a", "b"); err == nil {
		t.Fatal("expected error")
	}
}

func TestLoopGuardDifferentPairs(t *testing.T) {
	lg := NewLoopGuard(1, 1)
	lg.Record("a", "b")
	if err := lg.Check("a", "c"); err != nil {
		t.Fatal(err)
	}
	if err := lg.Check("c", "b"); err != nil {
		t.Fatal(err)
	}
}

func TestLoopGuardWindowExpiry(t *testing.T) {
	lg := NewLoopGuard(1, 0)
	lg.Record("a", "b")
	time.Sleep(20 * time.Millisecond)
	if err := lg.Check("a", "b"); err != nil {
		t.Fatal(err)
	}
}
