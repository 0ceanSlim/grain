package core

import (
	"context"
	"sort"
	"sync"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// NIP-66 self-discovering relay pool (issue #104, Phase 1: discovery).
//
// The known-relays browser must not be sourced from the mailbox union (every
// relay anyone ever listed — dead, typo'd, unbounded). Instead grain discovers
// NIP-66 monitors (kind 10166) and reads the relay records they publish (kind
// 30166), which carry liveness + capability metadata. This is Phase 1: find
// monitors, pull their relay sets, expose them. Consensus/trust across monitors
// (Phase 2) and staleness health-rolling (Phase 3) build on the cache below —
// which is why per-monitor attribution is retained rather than flattened.

// monitorDiscovery caches discovered monitors and their per-relay reports.
// relays is keyed URL -> monitor pubkey -> record, so a relay reported by three
// monitors keeps three records and a later consensus pass can count them.
type monitorDiscovery struct {
	mu       sync.RWMutex
	monitors map[string]*RelayMonitor
	relays   map[string]map[string]*DiscoveredRelay
	updated  time.Time
}

func newMonitorDiscovery() *monitorDiscovery {
	return &monitorDiscovery{
		monitors: make(map[string]*RelayMonitor),
		relays:   make(map[string]map[string]*DiscoveredRelay),
	}
}

// putMonitor stores the newest announcement per monitor. Returns true when the
// monitor is newly known (for the "new this round" count).
func (md *monitorDiscovery) putMonitor(m *RelayMonitor) bool {
	md.mu.Lock()
	defer md.mu.Unlock()
	cur, existed := md.monitors[m.Pubkey]
	if existed && cur.CreatedAt >= m.CreatedAt {
		return false
	}
	md.monitors[m.Pubkey] = m
	return !existed
}

// putRelay stores the newest 30166 record per (URL, monitor). Returns true when
// this monitor's report for the URL is newly seen.
func (md *monitorDiscovery) putRelay(d *DiscoveredRelay) bool {
	md.mu.Lock()
	defer md.mu.Unlock()
	byMon, ok := md.relays[d.URL]
	if !ok {
		byMon = make(map[string]*DiscoveredRelay)
		md.relays[d.URL] = byMon
	}
	cur, existed := byMon[d.MonitorPubkey]
	if existed && cur.ObservedAt >= d.ObservedAt {
		return false
	}
	byMon[d.MonitorPubkey] = d
	return !existed
}

func (md *monitorDiscovery) monitorPubkeys() []string {
	md.mu.RLock()
	defer md.mu.RUnlock()
	out := make([]string, 0, len(md.monitors))
	for pk := range md.monitors {
		out = append(out, pk)
	}
	return out
}

func (md *monitorDiscovery) monitorCount() int {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return len(md.monitors)
}

func (md *monitorDiscovery) relayCount() int {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return len(md.relays)
}

func (md *monitorDiscovery) stamp() {
	md.mu.Lock()
	md.updated = time.Now()
	md.mu.Unlock()
}

// merged collapses the per-monitor records to one view per URL: the freshest
// report, annotated with MonitorCount (how many distinct monitors reported it).
// Sorted by URL for stable output.
func (md *monitorDiscovery) merged() []DiscoveredRelayView {
	md.mu.RLock()
	defer md.mu.RUnlock()
	out := make([]DiscoveredRelayView, 0, len(md.relays))
	for _, byMon := range md.relays {
		var freshest *DiscoveredRelay
		for _, rec := range byMon {
			if freshest == nil || rec.ObservedAt > freshest.ObservedAt {
				freshest = rec
			}
		}
		if freshest == nil {
			continue
		}
		out = append(out, DiscoveredRelayView{DiscoveredRelay: *freshest, MonitorCount: len(byMon)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URL < out[j].URL })
	return out
}

// DiscoveredRelayView is one relay's merged discovery record for the browser:
// the freshest monitor report plus how many monitors reported it. MonitorCount
// is what a Phase 2 consensus filter (require >= K) will key on.
type DiscoveredRelayView struct {
	DiscoveredRelay
	MonitorCount int `json:"monitor_count"`
}

// discoverySources are the relays grain probes for NIP-66 events: index relays
// plus whatever's currently connected. Per NIP-66 testing, dedicated indexers
// often don't carry 10166/30166 while general relays reached via resolved lists
// do — so the connected/general set is the substrate where monitors surface.
func (c *Client) discoverySources() []string {
	return appendUnique(c.indexRelays(), c.GetConnectedRelays())
}

// DiscoverMonitors queries kind-10166 announcements across the discovery sources
// and folds them into the monitor pool. One monitor is enough to bootstrap.
// Returns the total number of monitors known after the pass.
func (c *Client) DiscoverMonitors(ctx context.Context) int {
	relays := c.discoverySources()
	if len(relays) == 0 {
		return c.discovery.monitorCount()
	}
	const limit = 200
	lim := limit
	events := c.FetchEvents(ctx,
		[]nostr.Filter{{Kinds: []int{KindRelayMonitorAnnouncement}, Limit: &lim}},
		relays, limit, 8*time.Second)

	added := 0
	for _, ev := range events {
		if m := parseMonitorAnnouncement(ev); m != nil && c.discovery.putMonitor(m) {
			added++
		}
	}
	clog().Info("NIP-66 monitor discovery",
		"queried_relays", len(relays), "events", len(events),
		"new_monitors", added, "total_monitors", c.discovery.monitorCount())
	return c.discovery.monitorCount()
}

// RefreshDiscoveredRelays pulls each known monitor's kind-30166 records into the
// cache. No-op (returns current count) when no monitors are known yet — call
// DiscoverMonitors first. Returns the distinct-relay count after the pass.
func (c *Client) RefreshDiscoveredRelays(ctx context.Context) int {
	monitors := c.discovery.monitorPubkeys()
	if len(monitors) == 0 {
		return c.discovery.relayCount()
	}
	relays := c.discoverySources()
	if len(relays) == 0 {
		return c.discovery.relayCount()
	}
	const limit = 2000
	lim := limit
	events := c.FetchEvents(ctx,
		[]nostr.Filter{{Kinds: []int{KindRelayDiscovery}, Authors: monitors, Limit: &lim}},
		relays, limit, 12*time.Second)

	added := 0
	for _, ev := range events {
		if d := parseRelayDiscovery(ev); d != nil && c.discovery.putRelay(d) {
			added++
		}
	}
	c.discovery.stamp()
	clog().Info("NIP-66 relay discovery",
		"monitors", len(monitors), "events", len(events),
		"new_records", added, "distinct_relays", c.discovery.relayCount())
	return c.discovery.relayCount()
}

// DiscoverRelays runs a full discovery pass: find monitors, then pull their
// relay sets. Safe to call periodically (Phase 3 will add staleness eviction).
func (c *Client) DiscoverRelays(ctx context.Context) {
	c.DiscoverMonitors(ctx)
	c.RefreshDiscoveredRelays(ctx)
}

// DiscoveredRelays returns the merged NIP-66 discovery set — one record per URL
// with its monitor count — for the known-relays browser. Empty until a
// discovery pass has run.
func (c *Client) DiscoveredRelays() []DiscoveredRelayView {
	return c.discovery.merged()
}

// DiscoveredRelayURLs returns just the URLs of the discovered set, for folding
// into the browser's known list.
func (c *Client) DiscoveredRelayURLs() []string {
	views := c.discovery.merged()
	out := make([]string, 0, len(views))
	for _, v := range views {
		out = append(out, v.URL)
	}
	return out
}
