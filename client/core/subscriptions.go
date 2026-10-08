package core

import (
	"sync"
	//"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// Subscription manages a Nostr subscription across multiple relays
type Subscription struct {
	ID         string
	Filters    []nostr.Filter
	Relays     []string
	Events     chan *nostr.Event
	Errors     chan error
	Done       chan struct{}
	EOSE       chan string // NEW: Channel for EOSE messages with relay URL
	client     *Client
	mu         sync.RWMutex
	active     bool
	closed     bool            // set under mu in Close before the channels are closed; gates RouteMessage sends
	eoseRelays map[string]bool // NEW: Track which relays sent EOSE
	acquired   []string        // relays this sub holds a pool lease on (released on Close)

	// owner is the user this subscription runs as (nil = anonymous). Relays
	// the owner has NIP-42 authenticated to are reached over the owner's own
	// connection; everything else over the Client's shared pool. pools pins
	// each relay's choice for the subscription's lifetime. Guarded by mu.
	owner *UserContext
	pools map[string]*RelayPool
}

// NewSubscription creates a new subscription instance
func NewSubscription(id string, filters []nostr.Filter, relays []string, client *Client) *Subscription {
	return &Subscription{
		ID:         id,
		Filters:    filters,
		Relays:     relays,
		Events:     make(chan *nostr.Event, 100), // Buffered channel
		Errors:     make(chan error, 10),
		Done:       make(chan struct{}),
		EOSE:       make(chan string, len(relays)), // NEW: Buffered for each relay
		client:     client,
		active:     false,
		eoseRelays: make(map[string]bool), // NEW: Initialize map
		pools:      make(map[string]*RelayPool),
	}
}

// poolFor returns the pool url is reached through, choosing it on first use
// and keeping that choice so REQ, CLOSE and Release all hit the same pool.
// Caller holds s.mu.
func (s *Subscription) poolFor(url string) *RelayPool {
	if p, ok := s.pools[url]; ok {
		return p
	}
	p := s.client.poolFor(s.owner, url)
	if p != s.client.relayPool {
		p.RegisterSubscription(s.ID, s)
	}
	s.pools[url] = p
	return p
}

// unregisterAll removes the subscription from every pool it registered with.
// Caller holds s.mu.
func (s *Subscription) unregisterAll() {
	s.client.relayPool.UnregisterSubscription(s.ID)
	for _, p := range s.pools {
		if p != s.client.relayPool {
			p.UnregisterSubscription(s.ID)
		}
	}
}

// Start begins the subscription on all specified relays
func (s *Subscription) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active {
		return &ClientError{Message: "subscription already active"}
	}

	clog().Debug("Starting subscription", "sub_id", s.ID, "relay_count", len(s.Relays))

	// Register with the shared pool for message routing, and pick each relay's
	// pool up front (registering with any per-user pool it lands on) so the
	// concurrent dials below only read s.pools.
	s.client.relayPool.RegisterSubscription(s.ID, s)
	pools := make([]*RelayPool, len(s.Relays))
	for i, relayURL := range s.Relays {
		pools[i] = s.poolFor(relayURL)
	}

	// Send REQ message to all relays
	reqMessage := []interface{}{"REQ", s.ID}
	for _, filter := range s.Filters {
		reqMessage = append(reqMessage, filter)
	}

	// Connect to all target relays CONCURRENTLY (bounded by the pool's dial
	// semaphore) rather than one at a time, so a slow or dead relay in someone's
	// outbox only costs its own dial timeout instead of stacking up serially
	// behind the others. Each goroutine writes its own index, so no shared-write
	// race; wg.Wait synchronises before we read the results.
	connected := make([]bool, len(s.Relays))
	var wg sync.WaitGroup
	for i, relayURL := range s.Relays {
		wg.Add(1)
		go func(i int, url string) {
			defer wg.Done()
			if _, err := pools[i].Acquire(url); err != nil {
				clog().Debug("Failed to acquire relay for subscription", "relay", url, "sub_id", s.ID, "error", err)
				return
			}
			connected[i] = true
		}(i, relayURL)
	}
	wg.Wait()

	// Fire the REQ to every relay that came up, holding its lease for the
	// subscription's lifetime so it isn't idle-evicted out from under us.
	var lastErr error
	sent := 0
	for i, relayURL := range s.Relays {
		if !connected[i] {
			continue
		}
		s.acquired = append(s.acquired, relayURL)

		if err := pools[i].SendMessage(relayURL, reqMessage); err != nil {
			// Demoted to Debug: races with upstream disconnect are normal
			// flakiness, not grain bugs.
			clog().Debug("Failed to send subscription to relay", "relay", relayURL, "sub_id", s.ID, "error", err)
			lastErr = err
			continue
		}

		// Mark relay as having this subscription
		if conn, err := pools[i].GetConnection(relayURL); err == nil {
			conn.mu.Lock()
			conn.Subscriptions[s.ID] = true
			conn.mu.Unlock()
		}

		sent++
	}

	if sent == 0 && lastErr != nil {
		// Unregister and release any leases taken before bailing out.
		s.unregisterAll()
		for _, relayURL := range s.acquired {
			s.poolFor(relayURL).Release(relayURL)
		}
		s.acquired = nil
		return lastErr
	}

	s.active = true

	// No need for processMessages goroutine - routing happens directly from readHandler

	clog().Info("Subscription started", "sub_id", s.ID, "sent_to", sent, "total_relays", len(s.Relays))
	return nil
}

// Update Close to close the EOSE channel too:
func (s *Subscription) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return nil
	}

	clog().Debug("Closing subscription", "sub_id", s.ID)

	// Unregister from every pool
	s.unregisterAll()

	// Send CLOSE to, and release the lease on, each relay this subscription
	// actually acquired (the connected subset — others never got a REQ). This
	// also lets the connections be idle-evicted once nothing else needs them.
	closeMessage := []interface{}{"CLOSE", s.ID}
	for _, relayURL := range s.acquired {
		pool := s.poolFor(relayURL)
		if err := pool.SendMessage(relayURL, closeMessage); err != nil {
			// Demoted to Debug: closing a sub on an already-disconnected relay
			// is expected during teardown, not a problem.
			clog().Debug("Failed to send close to relay", "relay", relayURL, "sub_id", s.ID, "error", err)
		}
		if conn, err := pool.GetConnection(relayURL); err == nil {
			conn.mu.Lock()
			delete(conn.Subscriptions, s.ID)
			conn.mu.Unlock()
		}
		pool.Release(relayURL)
	}
	s.acquired = nil

	s.active = false
	// Mark closed before closing the channels so a concurrent RouteMessage —
	// which takes s.mu.RLock() and checks this flag — either completes its send
	// before we close (it holds RLock, our Lock waits) or sees closed and drops
	// the message. Without this, a late EVENT racing Close panicked with "send
	// on closed channel" (the select/default guards a full channel, not a closed
	// one).
	s.closed = true
	close(s.Done)
	close(s.Events)
	close(s.Errors)
	close(s.EOSE) // NEW: Close EOSE channel

	clog().Debug("Subscription closed", "sub_id", s.ID)
	return nil
}

// AddRelay adds a new relay to an active subscription
func (s *Subscription) AddRelay(url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if relay is already in the list
	for _, existingURL := range s.Relays {
		if existingURL == url {
			return &ClientError{Message: "relay already in subscription"}
		}
	}

	// Add to relay list
	s.Relays = append(s.Relays, url)

	// If subscription is active, send REQ to new relay
	if s.active {
		// Connect-on-demand and hold a lease, mirroring Start.
		pool := s.poolFor(url)
		if _, err := pool.Acquire(url); err != nil {
			s.Relays = s.Relays[:len(s.Relays)-1]
			return err
		}
		s.acquired = append(s.acquired, url)

		reqMessage := []interface{}{"REQ", s.ID}
		for _, filter := range s.Filters {
			reqMessage = append(reqMessage, filter)
		}

		if err := pool.SendMessage(url, reqMessage); err != nil {
			// Remove from list and drop the lease if send failed
			s.Relays = s.Relays[:len(s.Relays)-1]
			s.acquired = s.acquired[:len(s.acquired)-1]
			pool.Release(url)
			return err
		}

		// Mark relay as having this subscription
		if conn, err := pool.GetConnection(url); err == nil {
			conn.mu.Lock()
			conn.Subscriptions[s.ID] = true
			conn.mu.Unlock()
		}
	}

	clog().Debug("Relay added to subscription", "sub_id", s.ID, "relay", url)
	return nil
}

// RemoveRelay removes a relay from the subscription
func (s *Subscription) RemoveRelay(url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Find and remove relay from list
	found := false
	for i, existingURL := range s.Relays {
		if existingURL == url {
			s.Relays = append(s.Relays[:i], s.Relays[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return &ClientError{Message: "relay not found in subscription"}
	}

	// If subscription is active, send CLOSE to removed relay
	if s.active {
		closeMessage := []interface{}{"CLOSE", s.ID}
		if err := s.poolFor(url).SendMessage(url, closeMessage); err != nil {
			clog().Warn("Failed to send close to removed relay", "relay", url, "sub_id", s.ID, "error", err)
		}

		// Remove subscription from relay
		if conn, err := s.poolFor(url).GetConnection(url); err == nil {
			conn.mu.Lock()
			delete(conn.Subscriptions, s.ID)
			conn.mu.Unlock()
		}
	}

	// Drop the lease if this sub was holding one for the removed relay.
	for i, u := range s.acquired {
		if u == url {
			s.acquired = append(s.acquired[:i], s.acquired[i+1:]...)
			s.poolFor(url).Release(url)
			break
		}
	}

	clog().Debug("Relay removed from subscription", "sub_id", s.ID, "relay", url)
	return nil
}

// processMessages handles incoming messages for this subscription
//func (s *Subscription) processMessages() {
//	// TODO: This will be implemented to process messages from relay read handlers
//	// For now, this is a placeholder that will be connected to relay message routing
//
//	ticker := time.NewTicker(30 * time.Second)
//	defer ticker.Stop()
//
//	for {
//		select {
//		case <-s.Done:
//			clog().Debug("Message processor stopped", "sub_id", s.ID)
//			return
//		case <-ticker.C:
//			// Periodic heartbeat - could be used for subscription health checks
//			clog().Debug("Subscription heartbeat", "sub_id", s.ID)
//		}
//	}
//}

// IsActive returns whether the subscription is currently active
func (s *Subscription) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

// GetRelayCount returns the number of relays in this subscription
func (s *Subscription) GetRelayCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Relays)
}

// GetFilters returns a copy of the subscription filters
func (s *Subscription) GetFilters() []nostr.Filter {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filters := make([]nostr.Filter, len(s.Filters))
	copy(filters, s.Filters)
	return filters
}
