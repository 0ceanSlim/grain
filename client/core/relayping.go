package core

import (
	"net"
	"net/url"
	"sync"
	"time"
)

type relayPingEntry struct {
	ms int // milliseconds; -1 = unreachable within the timeout
	at time.Time
}

const (
	relayPingTTL     = 2 * time.Minute
	relayPingTimeout = 3 * time.Second
)

// PingRelay measures TCP-connect latency to a relay — a cheap reachability/
// latency probe, not a WebSocket or NIP-01 round-trip — and TTL-caches it.
// Returns milliseconds, or -1 if the host can't be reached within the timeout
// (also cached, so onion/unroutable hosts don't stall every sort). Used by the
// known-relays "fastest first" ordering.
func (c *Client) PingRelay(relayURL string) int {
	if norm, ok := normalizeRelayURL(relayURL); ok {
		relayURL = norm
	}
	c.relayPingMu.Lock()
	if e, ok := c.relayPingCache[relayURL]; ok && time.Since(e.at) < relayPingTTL {
		c.relayPingMu.Unlock()
		return e.ms
	}
	c.relayPingMu.Unlock()

	ms := tcpPing(relayURL)

	c.relayPingMu.Lock()
	c.relayPingCache[relayURL] = relayPingEntry{ms: ms, at: time.Now()}
	c.relayPingMu.Unlock()
	return ms
}

// PingRelays pings a set of relays in parallel (bounded worker pool), returning
// url -> ms (-1 for unreachable). The relay manager calls this for just the rows
// currently in view, so the set is small even though the known set is large.
func (c *Client) PingRelays(urls []string) map[string]int {
	const workers = 16
	out := make(map[string]int, len(urls))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for _, u := range urls {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			ms := c.PingRelay(u)
			mu.Lock()
			out[u] = ms
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

// tcpPing dials TCP to the relay's host:port and returns the connect time in
// milliseconds, or -1 on failure/timeout. wss defaults to :443, ws to :80.
func tcpPing(wsURL string) int {
	u, err := url.Parse(wsURL)
	if err != nil {
		return -1
	}
	host := u.Hostname()
	if host == "" {
		return -1
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "wss" {
			port = "443"
		} else {
			port = "80"
		}
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), relayPingTimeout)
	if err != nil {
		return -1
	}
	_ = conn.Close()
	return max(int(time.Since(start).Milliseconds()), 0)
}
