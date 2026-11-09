package accounts

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// Hashing is capped by a semaphore so a burst of sign-ins cannot exhaust
// memory: with one slot, no two hashes run at once.
func TestHashingIsCapped(t *testing.T) {
	old := hashGate
	hashGate = make(chan struct{}, 1)
	defer func() { hashGate = old }()

	var inFlight, peak atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = withHashSlot(context.Background(), func() {
				n := inFlight.Add(1)
				for {
					p := peak.Load()
					if n <= p || peak.CompareAndSwap(p, n) {
						break
					}
				}
				_, _ = hashPassword("a long enough password")
				inFlight.Add(-1)
			})
		}()
	}
	wg.Wait()
	if peak.Load() != 1 {
		t.Errorf("peak concurrent hashes %d with one slot", peak.Load())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	hashGate <- struct{}{} // the slot is taken
	if err := withHashSlot(ctx, func() {}); err == nil {
		t.Error("a cancelled request waited for a slot")
	}
	<-hashGate
}
