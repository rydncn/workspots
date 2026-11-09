package web

import (
	"testing"
	"time"
)

func TestLimiterForgetsKeysWhoseWindowHasPassed(t *testing.T) {
	t.Parallel()
	l := newLimiter(10 * time.Millisecond)
	l.record("signin:addr:someone@example.test")
	l.record("signin:addr:someone@example.test")
	if l.retryAfter("signin:addr:someone@example.test", 2) == 0 || l.size() != 1 {
		t.Fatal("two events in the window should hit a limit of two")
	}
	time.Sleep(15 * time.Millisecond)
	if l.retryAfter("signin:addr:someone@example.test", 2) != 0 || l.size() != 0 {
		t.Errorf("stale key still held: size %d", l.size())
	}
}
