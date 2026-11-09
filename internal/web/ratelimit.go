package web

import (
	"strconv"
	"sync"
	"time"
)

// limiter counts events per key in a sliding window, in memory: one process,
// which is all this deployment has.
// shortcut: counters are per process and reset on restart, move to the database
// when there is more than one instance.
type limiter struct {
	mu     sync.Mutex
	window time.Duration
	events map[string][]time.Time
}

func newLimiter(window time.Duration) *limiter {
	return &limiter{window: window, events: map[string][]time.Time{}}
}

// retryAfter returns 0 when key is under limit, else the seconds until the
// oldest counted event leaves the window.
func (l *limiter) retryAfter(key string, limit int) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	kept := l.events[key][:0]
	for _, t := range l.events[key] {
		if now.Sub(t) < l.window {
			kept = append(kept, t)
		}
	}
	l.events[key] = kept
	if len(kept) < limit {
		return 0
	}
	return int(l.window.Seconds()-now.Sub(kept[0]).Seconds()) + 1
}

func (l *limiter) record(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events[key] = append(l.events[key], time.Now())
}

func retryHeader(seconds int) string { return strconv.Itoa(seconds) }
