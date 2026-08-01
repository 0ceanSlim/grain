package core

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMonitorDiscoveryCache(t *testing.T) {
	md := newMonitorDiscovery()

	// New monitor is added; an older re-announcement is ignored.
	if !md.putMonitor(&RelayMonitor{Pubkey: "m1", CreatedAt: 100}) {
		t.Error("first monitor should count as new")
	}
	if md.putMonitor(&RelayMonitor{Pubkey: "m1", CreatedAt: 50}) {
		t.Error("older announcement should not count as new")
	}
	if md.monitorCount() != 1 {
		t.Errorf("monitor count = %d, want 1", md.monitorCount())
	}

	// Two monitors report the same relay: one merged view, MonitorCount 2, the
	// freshest report (by ObservedAt) wins. A third URL from one monitor is its
	// own entry.
	md.putRelay(&DiscoveredRelay{URL: "wss://a", MonitorPubkey: "m1", ObservedAt: 100, Network: "clearnet"})
	md.putRelay(&DiscoveredRelay{URL: "wss://a", MonitorPubkey: "m2", ObservedAt: 200, Network: "tor"})
	md.putRelay(&DiscoveredRelay{URL: "wss://b", MonitorPubkey: "m1", ObservedAt: 100})

	if md.relayCount() != 2 {
		t.Errorf("distinct relays = %d, want 2", md.relayCount())
	}
	merged := md.merged()
	if len(merged) != 2 || merged[0].URL != "wss://a" { // sorted by URL
		t.Fatalf("merged = %+v", merged)
	}
	if merged[0].MonitorCount != 2 {
		t.Errorf("monitor count = %d, want 2", merged[0].MonitorCount)
	}
	if merged[0].Network != "tor" { // ObservedAt 200 > 100
		t.Errorf("freshest report should win, got network %q", merged[0].Network)
	}

	// A newer report from a monitor replaces only its own prior report; the
	// distinct-monitor count is unchanged.
	md.putRelay(&DiscoveredRelay{URL: "wss://a", MonitorPubkey: "m1", ObservedAt: 300, Network: "i2p"})
	if got := md.merged()[0]; got.Network != "i2p" || got.MonitorCount != 2 {
		t.Errorf("newer same-monitor report: network=%q count=%d (want i2p/2)", got.Network, got.MonitorCount)
	}
}

func TestMonitorDiscoveryConsensus(t *testing.T) {
	// N=1 (bootstrap): a single monitor's relays surface (K=1) — nothing else
	// to corroborate against yet.
	single := newMonitorDiscovery()
	single.putRelay(&DiscoveredRelay{URL: "wss://a", MonitorPubkey: "m1", RTTOpen: -1})
	single.putRelay(&DiscoveredRelay{URL: "wss://b", MonitorPubkey: "m1", RTTOpen: -1})
	if got := single.consensus(); len(got) != 2 {
		t.Errorf("N=1: expected both relays (K=1), got %d", len(got))
	}

	// N=2: K=2 — only relays BOTH monitors report survive; a single-monitor
	// relay is not trusted.
	two := newMonitorDiscovery()
	two.putRelay(&DiscoveredRelay{URL: "wss://shared", MonitorPubkey: "m1"})
	two.putRelay(&DiscoveredRelay{URL: "wss://shared", MonitorPubkey: "m2"})
	two.putRelay(&DiscoveredRelay{URL: "wss://solo", MonitorPubkey: "m1"})
	got := two.consensus()
	if len(got) != 1 || got[0].URL != "wss://shared" || got[0].MonitorCount != 2 {
		t.Fatalf("N=2: expected only the shared relay (count 2), got %+v", got)
	}

	// Outlier discard: A and B agree on x/y/z; C spams 10 relays no one else
	// sees. C (0% corroboration) is discarded, so its spam never surfaces and
	// the A/B set remains.
	md := newMonitorDiscovery()
	for _, u := range []string{"wss://x", "wss://y", "wss://z"} {
		md.putRelay(&DiscoveredRelay{URL: u, MonitorPubkey: "A"})
		md.putRelay(&DiscoveredRelay{URL: u, MonitorPubkey: "B"})
	}
	for i := 0; i < 10; i++ {
		md.putRelay(&DiscoveredRelay{URL: "wss://spam" + strconv.Itoa(i), MonitorPubkey: "C"})
	}
	res := md.consensus()
	if len(res) != 3 {
		t.Fatalf("outlier: expected 3 consensus relays, got %d", len(res))
	}
	for _, r := range res {
		if strings.HasPrefix(r.URL, "wss://spam") {
			t.Errorf("outlier monitor's spam relay leaked: %s", r.URL)
		}
		if r.MonitorCount != 2 {
			t.Errorf("relay %s count = %d, want 2", r.URL, r.MonitorCount)
		}
	}
}

func TestMonitorDiscoveryEvictStale(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	md := newMonitorDiscovery()

	// Fresh: last 30166 is "now" — kept. Stale: last 30166 is 3h old, past the
	// 2x grace on a 1h frequency — evicted with its relay. No-records: kept
	// (nothing to judge, contributes nothing until it publishes).
	md.putMonitor(&RelayMonitor{Pubkey: "fresh", Frequency: 3600})
	md.putRelay(&DiscoveredRelay{URL: "wss://fresh", MonitorPubkey: "fresh", ObservedAt: now.Unix()})
	md.putMonitor(&RelayMonitor{Pubkey: "stale", Frequency: 3600})
	md.putRelay(&DiscoveredRelay{URL: "wss://stale", MonitorPubkey: "stale", ObservedAt: now.Add(-3 * time.Hour).Unix()})
	md.putMonitor(&RelayMonitor{Pubkey: "norecords", Frequency: 3600})

	if evicted := md.evictStale(now, staleGraceFactor); evicted != 1 {
		t.Fatalf("expected 1 eviction, got %d", evicted)
	}
	if md.monitorCount() != 2 {
		t.Errorf("monitors remaining = %d, want 2 (fresh + norecords)", md.monitorCount())
	}
	if got := md.merged(); len(got) != 1 || got[0].URL != "wss://fresh" {
		t.Errorf("after eviction expected only wss://fresh, got %+v", got)
	}
}
