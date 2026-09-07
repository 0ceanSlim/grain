package core

import (
	"fmt"
	"sync"
	"testing"
)

// TestRouteMessageDoesNotPanicOnClose is a regression test for the
// "send on closed channel" panic that crashed the relay under subscription
// churn (every grain-test-* instance in CI panicked from
// client/core/relays.go:155). Subscription.Close() closes Events/EOSE/Errors
// while the relay read handler concurrently routes late messages into them;
// the select/default on each send guards a *full* channel, not a *closed* one,
// so the send panicked.
//
// The fix serializes the two: Close sets sub.closed under sub.mu.Lock() before
// closing the channels, and RouteMessage takes sub.mu.RLock() and drops the
// message if closed. This test drives many senders against a racing Close over
// many subscription lifecycles; a regression panics (crashing the test binary),
// and -race would flag any unsynchronized access to the flag or channels.
func TestRouteMessageDoesNotPanicOnClose(t *testing.T) {
	const (
		iterations = 100
		senders    = 4
		perSender  = 200
	)

	for iter := 0; iter < iterations; iter++ {
		c := NewClient(DefaultConfig())
		sub := NewSubscription(fmt.Sprintf("race-%d", iter), nil, nil, c)
		c.relayPool.RegisterSubscription(sub.ID, sub)

		// Mark active so Close() runs its full teardown (acquired is empty, so
		// it sends no network messages) and actually closes the channels.
		sub.mu.Lock()
		sub.active = true
		sub.mu.Unlock()

		// Drain the consumer side so buffered channels don't just fill up —
		// we want sends to keep reaching the channel op, racing the close.
		stop := make(chan struct{})
		go func() {
			for {
				select {
				case <-stop:
					return
				case <-sub.Events:
				case <-sub.EOSE:
				case <-sub.Errors:
				}
			}
		}()

		var wg sync.WaitGroup
		for g := 0; g < senders; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < perSender; i++ {
					c.relayPool.messageRouter.RouteMessage(sub.ID, "EOSE", nil, "wss://example")
					c.relayPool.messageRouter.RouteMessage(sub.ID, "CLOSED", nil, "wss://example")
				}
			}()
		}

		// Race Close against the in-flight sends.
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sub.Close()
		}()

		wg.Wait()
		close(stop)
	}
}
