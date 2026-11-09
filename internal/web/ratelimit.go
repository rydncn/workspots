package web

import (
	"crypto/sha256"
	"strconv"
	"sync"
	"time"
)

// limiter counts events per key in a sliding window, in memory: one process,
// which is all this deployment has. Keys are stored hashed (they hold
// addresses and IPs) and dropped once nothing in the window remains.
// shortcut: counters are per process and reset on restart, move to the database
// when there is more than one instance.
type limiter struct {
	mu      sync.Mutex
	window  time.Duration
	events  map[[32]byte][]time.Time
	records int
}

func newLimiter(window time.Duration) *limiter {
	return &limiter{window: window, events: map[[32]byte][]time.Time{}}
}

// retryAfter returns 0 when key is under limit, else the seconds until the
// oldest counted event leaves the window.
func (l *limiter) retryAfter(key string, limit int) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := l.prune(sha256.Sum256([]byte(key)), time.Now())
	if len(kept) < limit {
		return 0
	}
	return int(l.window.Seconds()-time.Since(kept[0]).Seconds()) + 1
}

func (l *limiter) record(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	h := sha256.Sum256([]byte(key))
	l.events[h] = append(l.events[h], time.Now())
	// Every so often, forget keys nobody has asked about since their window passed.
	if l.records++; l.records%1000 == 0 {
		now := time.Now()
		for k := range l.events {
			l.prune(k, now)
		}
	}
}

// prune drops events outside the window for k, deleting k when none remain.
// The caller holds the lock.
func (l *limiter) prune(k [32]byte, now time.Time) []time.Time {
	kept := l.events[k][:0]
	for _, t := range l.events[k] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	if len(kept) == 0 {
		delete(l.events, k)
		return nil
	}
	l.events[k] = kept
	return kept
}

// size is for tests.
func (l *limiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.events)
}

func retryHeader(seconds int) string { return strconv.Itoa(seconds) }
