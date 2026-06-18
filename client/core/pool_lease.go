package core

import (
	"context"
	"fmt"
	"net"
	"sort"
	"time"

	"golang.org/x/net/websocket"
)

// realDial opens a websocket to url with a dial timeout. It is the default
// RelayPool.dialFn; tests inject a fake so Acquire can be exercised without a
// live relay.
func realDial(url string, timeout time.Duration) (*websocket.Conn, error) {
	cfg, err := websocket.NewConfig(url, "http://localhost/")
	if err != nil {
		return nil, fmt.Errorf("failed to create config for relay %s: %w", url, err)
	}
	cfg.Dialer = &net.Dialer{Timeout: timeout}
	conn, err := websocket.DialConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to relay %s: %w", url, err)
	}
	return conn, nil
}

// Pin marks urls so the idle sweeper never evicts them — used for the index/
// seed relays that must stay connected to resolve relay lists for anyone. It
// does not dial; Acquire still establishes the connection on demand.
func (rp *RelayPool) Pin(urls ...string) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	for _, u := range urls {
		rp.pinned[u] = true
	}
}

// Acquire returns a connected RelayConnection for url, dialing on demand, and
// takes a lease on it. Every successful Acquire must be balanced by a Release
// so the connection can eventually be idle-evicted. Concurrent Acquires for the
// same url share a single dial (single-flight) and the resulting connection.
func (rp *RelayPool) Acquire(url string) (*RelayConnection, error) {
	openTimeout := rp.config.OpenTimeout
	if openTimeout <= 0 {
		openTimeout = rp.config.ConnectionTimeout
	}
	return rp.acquire(url, openTimeout, false)
}

// EnsureConnectedForSend makes sure url is connected so the publish path can send
// to it, redialing a dropped target relay bounded by timeout. It ignores dial
// backoff because a publish is an explicit user action (unlike a background
// outbox dial that should fail fast), and it does not retain the lease — the
// caller only needs the pooled connection for the immediate send, and the
// sweeper evicts it later if it stays idle. Returns an error if it can't connect
// in time, which the broadcast surfaces as a per-relay timeout.
func (rp *RelayPool) EnsureConnectedForSend(url string, timeout time.Duration) error {
	if _, err := rp.acquire(url, timeout, true); err != nil {
		return err
	}
	rp.Release(url)
	return nil
}

// acquire is the shared dial-and-lease core behind Acquire and
// EnsureConnectedForSend. openTimeout bounds the dial; bypassBackoff skips the
// dial-backoff gate.
func (rp *RelayPool) acquire(url string, openTimeout time.Duration, bypassBackoff bool) (*RelayConnection, error) {
	normalized, ok := normalizeRelayURL(url)
	if !ok {
		return nil, fmt.Errorf("invalid relay url: %q", url)
	}
	url = normalized

	for {
		rp.mu.Lock()

		if conn, ok := rp.connections[url]; ok {
			if conn.Status == StatusConnected {
				// Fast path: reuse the live connection, take a lease.
				conn.mu.Lock()
				conn.leases++
				conn.idleAt = time.Time{}
				conn.mu.Unlock()
				rp.mu.Unlock()
				return conn, nil
			}
			// A dead/errored connection is lingering — drop it and redial.
			delete(rp.connections, url)
		}

		if !bypassBackoff {
			if t, ok := rp.backoff[url]; ok && time.Now().Before(t) {
				rp.mu.Unlock()
				return nil, fmt.Errorf("relay %s in dial backoff", url)
			}
		}

		// Single-flight: if a dial for this url is already in progress, wait for
		// it to finish and then retry the whole check (it may now be connected,
		// failed into backoff, or evicted).
		if ch, ok := rp.dialing[url]; ok {
			rp.mu.Unlock()
			<-ch
			continue
		}

		// We own the dial. Publish the in-flight marker so others wait on us.
		ch := make(chan struct{})
		rp.dialing[url] = ch
		rp.mu.Unlock()

		// Dial without holding the pool lock, bounded by the dial semaphore so a
		// burst of outbox lookups can't open unbounded sockets at once. The dial
		// timeout is the caller's: short for background outbox dials (fail fast),
		// longer for an explicit publish reconnect.
		rp.dialSem <- struct{}{}
		conn, err := rp.dialFn(url, openTimeout)
		<-rp.dialSem

		rp.mu.Lock()
		delete(rp.dialing, url)
		close(ch)

		if err != nil {
			rp.recordFailureLocked(url)
			rp.mu.Unlock()
			clog().Debug("Dial failed", "relay", url, "error", err)
			return nil, err
		}

		// Dial succeeded — clear any failure history.
		delete(rp.backoff, url)
		delete(rp.failCount, url)

		// Soft cap: if we're at capacity, evict the longest-idle unpinned,
		// lease-free connection to make room for this one.
		if len(rp.connections) >= rp.config.MaxConnections {
			if victim := rp.lruIdleVictimLocked(); victim != "" {
				rp.closeAndRemoveLocked(victim)
			}
		}

		rc := &RelayConnection{
			URL:           url,
			Conn:          conn,
			Status:        StatusConnected,
			LastPing:      time.Now(),
			Subscriptions: make(map[string]bool),
			writeChan:     make(chan []byte, 100),
			done:          make(chan struct{}),
			messageRouter: rp.messageRouter,
			leases:        1,
		}
		rp.connections[url] = rc
		rp.mu.Unlock()

		rp.startReader(rc)
		clog().Debug("Acquired relay connection", "relay", url)
		return rc, nil
	}
}

// Release drops one lease on url's connection. When the last lease is released
// the connection is marked idle (eligible for the sweeper) but not closed, so a
// later Acquire can reuse it. Releasing an unknown url, or one already at zero
// leases, is a safe no-op — this guards against the lease-counter underflow
// class of bug.
func (rp *RelayPool) Release(url string) {
	rp.mu.RLock()
	conn, ok := rp.connections[url]
	rp.mu.RUnlock()
	if !ok {
		return
	}
	conn.mu.Lock()
	if conn.leases > 0 {
		conn.leases--
	}
	if conn.leases == 0 {
		conn.idleAt = time.Now()
	}
	conn.mu.Unlock()
}

// recordFailureLocked grows url's dial backoff exponentially (base * 2^(n-1),
// capped at BackoffMax). Caller holds mu.
func (rp *RelayPool) recordFailureLocked(url string) {
	rp.failCount[url]++

	base := rp.config.BackoffBase
	if base <= 0 {
		base = 2 * time.Second
	}
	ceiling := rp.config.BackoffMax
	if ceiling <= 0 {
		ceiling = 60 * time.Second
	}

	d := base
	for i := 1; i < rp.failCount[url]; i++ {
		d *= 2
		if d >= ceiling {
			d = ceiling
			break
		}
	}
	rp.backoff[url] = time.Now().Add(d)
}

// lruIdleVictimLocked returns the url of the unpinned, lease-free connection
// that has been idle the longest, or "" if there is none. Caller holds mu.
func (rp *RelayPool) lruIdleVictimLocked() string {
	var victim string
	var oldest time.Time
	for url, conn := range rp.connections {
		if rp.pinned[url] {
			continue
		}
		conn.mu.RLock()
		idle := conn.leases == 0 && !conn.idleAt.IsZero()
		at := conn.idleAt
		conn.mu.RUnlock()
		if !idle {
			continue
		}
		if victim == "" || at.Before(oldest) {
			victim, oldest = url, at
		}
	}
	return victim
}

// closeAndRemoveLocked tears down and forgets a connection. Caller holds mu.
func (rp *RelayPool) closeAndRemoveLocked(url string) {
	if conn, ok := rp.connections[url]; ok {
		_ = conn.close()
		delete(rp.connections, url)
		clog().Debug("Evicted relay connection", "relay", url)
	}
}

// evictIdle closes and removes every unpinned, lease-free connection that has
// been idle longer than idleTTL. Returns how many were evicted.
func (rp *RelayPool) evictIdle(now time.Time, idleTTL time.Duration) int {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	var victims []string
	for url, conn := range rp.connections {
		if rp.pinned[url] {
			continue
		}
		conn.mu.RLock()
		evict := conn.leases == 0 && !conn.idleAt.IsZero() && now.Sub(conn.idleAt) > idleTTL
		conn.mu.RUnlock()
		if evict {
			victims = append(victims, url)
		}
	}
	for _, url := range victims {
		rp.closeAndRemoveLocked(url)
	}
	return len(victims)
}

// PoolStats is a snapshot of the relay pool's connection counts, for status /
// observability (e.g. an "x / y relays connected" dashboard indicator).
type PoolStats struct {
	Known     int `json:"known"`     // relays the client is aware of (defaults + resolved lists + connections)
	Total     int `json:"total"`     // relays tracked in the pool (have a connection slot)
	Connected int `json:"connected"` // currently connected
	Pinned    int `json:"pinned"`    // index/seed relays kept alive
	Leased    int `json:"leased"`    // connections with at least one active lease
}

// Stats returns a snapshot of the pool's connection counts (without Known,
// which the Client fills in since it spans the directory too).
func (rp *RelayPool) Stats() PoolStats {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	s := PoolStats{Total: len(rp.connections), Pinned: len(rp.pinned)}
	for _, conn := range rp.connections {
		conn.mu.RLock()
		if conn.Status == StatusConnected {
			s.Connected++
		}
		if conn.leases > 0 {
			s.Leased++
		}
		conn.mu.RUnlock()
	}
	return s
}

// allURLs returns every relay URL currently tracked in the pool.
func (rp *RelayPool) allURLs() []string {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	out := make([]string, 0, len(rp.connections))
	for u := range rp.connections {
		out = append(out, u)
	}
	return out
}

// PoolStats returns a snapshot of the pool's counts plus Known — the distinct
// relays the client is aware of: configured defaults, every relay tracked in
// the pool, and every relay from the indexer-seeded mailbox lists the directory
// has resolved. Known climbs as you interact; connections grow on demand.
func (c *Client) PoolStats() PoolStats {
	s := c.relayPool.Stats()
	s.Known = len(c.knownSet())
	return s
}

// knownSet is the distinct relays the client is aware of: the effective index
// seeds, every relay in the pool, and every relay the directory has resolved.
func (c *Client) knownSet() map[string]struct{} {
	known := make(map[string]struct{})
	for _, u := range c.indexRelays() {
		known[u] = struct{}{}
	}
	for _, u := range c.relayPool.allURLs() {
		known[u] = struct{}{}
	}
	for _, u := range c.directory.KnownRelays() {
		known[u] = struct{}{}
	}
	return known
}

// KnownRelays returns the known set as a sorted slice — the relays behind
// PoolStats.Known, for the known-relays browser.
func (c *Client) KnownRelays() []string {
	known := c.knownSet()
	out := make([]string, 0, len(known))
	for u := range known {
		out = append(out, u)
	}
	sort.Strings(out)
	return out
}

// RelayLiveStatus is the pool's live view of one relay for the known-relays
// browser. The zero value means "known but not currently in the pool".
type RelayLiveStatus struct {
	Connected bool `json:"connected"`
	Pinned    bool `json:"pinned"`
	Leased    bool `json:"leased"`
}

// StatusOf returns the pool's live status for url.
func (rp *RelayPool) StatusOf(url string) RelayLiveStatus {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	st := RelayLiveStatus{Pinned: rp.pinned[url]}
	if conn, ok := rp.connections[url]; ok {
		conn.mu.RLock()
		st.Connected = conn.Status == StatusConnected
		st.Leased = conn.leases > 0
		conn.mu.RUnlock()
	}
	return st
}

// KnownRelayStatus pairs a known relay URL with its live pool status, for the
// known-relays browser.
type KnownRelayStatus struct {
	URL string `json:"url"`
	RelayLiveStatus
}

// KnownRelaysWithStatus returns every known relay (sorted) annotated with its
// live pool status. NIP-11 detail is fetched separately, lazily, per relay.
func (c *Client) KnownRelaysWithStatus() []KnownRelayStatus {
	urls := c.KnownRelays()
	out := make([]KnownRelayStatus, 0, len(urls))
	for _, u := range urls {
		out = append(out, KnownRelayStatus{URL: u, RelayLiveStatus: c.relayPool.StatusOf(u)})
	}
	return out
}

// StartEvictionSweeper runs evictIdle on an interval until ctx is cancelled,
// bounded to the caller's lifetime like the relay health check (#93).
func (rp *RelayPool) StartEvictionSweeper(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		clog().Info("Relay pool eviction sweeper started", "interval", interval)
		for {
			select {
			case <-ctx.Done():
				clog().Info("Relay pool eviction sweeper stopping")
				return
			case <-ticker.C:
				if n := rp.evictIdle(time.Now(), rp.config.IdleTTL); n > 0 {
					clog().Debug("Idle relay connections evicted", "count", n)
				}
			}
		}
	}()
}
