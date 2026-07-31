package core

import "testing"

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
