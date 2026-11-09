package web

import (
	"testing"
	"time"
)

func TestLimiterForgetsKeysWhoseWindowHasPassed(t *testing.T) {
	t.Parallel()
	l := newLimiter(10 * time.Millisecond)
	if l.take("signin:addr:someone@example.test", 2) != 0 || l.take("signin:addr:someone@example.test", 2) != 0 {
		t.Fatal("the first two attempts are within a limit of two")
	}
	if l.take("signin:addr:someone@example.test", 2) == 0 || l.size() != 1 {
		t.Fatal("the third attempt should be refused")
	}
	time.Sleep(15 * time.Millisecond)
	if l.take("signin:addr:someone@example.test", 2) != 0 || l.size() != 1 {
		t.Errorf("stale attempts still counted: size %d", l.size())
	}
	l.refund("signin:addr:someone@example.test")
	if l.size() != 0 {
		t.Errorf("refund of the only attempt should drop the key: size %d", l.size())
	}
}
