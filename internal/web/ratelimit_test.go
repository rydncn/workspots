package web

import (
	"testing"
	"time"
)

func TestLimiterForgetsKeysWhoseWindowHasPassed(t *testing.T) {
	t.Parallel()
	l := newLimiter(10 * time.Millisecond)
	for i := 0; i < 2; i++ {
		if l.take("signin:addr:someone@example.test", 2) != 0 {
			t.Fatalf("attempt %d is within a limit of two", i+1)
		}
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

// A request the per-IP key refuses must not be counted against a later key:
// an address key is unbounded in cardinality, so otherwise a client past its
// IP quota could grow the limiter by one entry per request (security audit).
func TestLimiterStopsAtTheFirstRefusal(t *testing.T) {
	t.Parallel()
	l := newLimiter(time.Minute)
	for i := 0; i < 10; i++ {
		if l.takeAll(keyLimit{"signin:ip:10.0.0.1", 10}) != 0 {
			t.Fatalf("attempt %d refused", i+1)
		}
	}
	for i := 0; i < 100; i++ {
		if l.takeAll(keyLimit{"signin:ip:10.0.0.1", 10}, keyLimit{"signin:addr:" + string(rune('a'+i%26)) + string(rune('a'+i/26)), 5}) == 0 {
			t.Fatalf("attempt %d past the IP quota was allowed", i+1)
		}
	}
	if n := l.size(); n != 1 {
		t.Errorf("%d keys held; refused requests allocated %d address keys", n, n-1)
	}
}
