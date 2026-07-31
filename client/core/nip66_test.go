package core

import (
	"testing"

	nostr "github.com/0ceanslim/grain/server/types"
)

func TestParseMonitorAnnouncement(t *testing.T) {
	// Wrong kind -> nil.
	if parseMonitorAnnouncement(&nostr.Event{Kind: 30166}) != nil {
		t.Fatal("non-10166 event should not parse as a monitor")
	}

	ev := &nostr.Event{
		Kind:      KindRelayMonitorAnnouncement,
		PubKey:    "monitor-pubkey",
		CreatedAt: 1700,
		Tags: [][]string{
			{"frequency", "3600"},
			{"c", "open"},
			{"c", "read"},
			{"c", "nip11"},
			{"g", "ww8p1r4t8"},
			{"timeout", "5000", "open"}, // extra columns ignored
			{"bogus"},                   // too short, skipped
		},
	}
	m := parseMonitorAnnouncement(ev)
	if m == nil {
		t.Fatal("expected a monitor")
	}
	if m.Pubkey != "monitor-pubkey" || m.CreatedAt != 1700 {
		t.Errorf("identity/time wrong: %+v", m)
	}
	if m.Frequency != 3600 {
		t.Errorf("frequency = %d, want 3600", m.Frequency)
	}
	if len(m.Checks) != 3 || m.Checks[0] != "open" || m.Checks[2] != "nip11" {
		t.Errorf("checks = %v", m.Checks)
	}
	if m.Geohash != "ww8p1r4t8" {
		t.Errorf("geohash = %q", m.Geohash)
	}
}

func TestParseRelayDiscovery(t *testing.T) {
	// Wrong kind -> nil.
	if parseRelayDiscovery(&nostr.Event{Kind: 10166}) != nil {
		t.Fatal("non-30166 event should not parse as a relay discovery")
	}
	// No usable URL (d is a hex pubkey, not a relay) -> nil.
	if parseRelayDiscovery(&nostr.Event{Kind: 30166, Tags: [][]string{{"d", "deadbeef"}}}) != nil {
		t.Fatal("a d-tag that isn't a relay URL should yield nil")
	}

	ev := &nostr.Event{
		Kind:      KindRelayDiscovery,
		PubKey:    "mon1",
		CreatedAt: 1800,
		Tags: [][]string{
			{"d", "wss://relay.example/"},
			{"n", "clearnet"},
			{"T", "PublicInbox"},
			{"N", "1"},
			{"N", "40"},
			{"R", "auth"},
			{"R", "!payment"},
			{"t", "nsfw"},
			{"k", "1"},
			{"k", "!4"}, // rejected kind — excluded from AcceptedKinds
			{"g", "ww8p1r4t8"},
			{"l", "en"},
			{"rtt-open", "234"},
			{"rtt-read", "150"},
			{"rtt-write", "200"},
		},
	}
	d := parseRelayDiscovery(ev)
	if d == nil {
		t.Fatal("expected a discovered relay")
	}
	if d.URL != "wss://relay.example" { // normalizeRelayURL strips the trailing slash
		t.Errorf("url = %q", d.URL)
	}
	if d.Network != "clearnet" || d.RelayType != "PublicInbox" {
		t.Errorf("network/type = %q/%q", d.Network, d.RelayType)
	}
	if len(d.SupportedNIPs) != 2 || d.SupportedNIPs[1] != 40 {
		t.Errorf("nips = %v", d.SupportedNIPs)
	}
	if len(d.Requirements) != 2 || d.Requirements[1] != "!payment" {
		t.Errorf("requirements = %v", d.Requirements)
	}
	if len(d.AcceptedKinds) != 1 || d.AcceptedKinds[0] != 1 {
		t.Errorf("accepted kinds = %v (rejected !4 must be excluded)", d.AcceptedKinds)
	}
	if d.Geohash != "ww8p1r4t8" || d.Language != "en" {
		t.Errorf("geo/lang = %q/%q", d.Geohash, d.Language)
	}
	if d.RTTOpen != 234 || d.RTTRead != 150 || d.RTTWrite != 200 {
		t.Errorf("rtt = %d/%d/%d", d.RTTOpen, d.RTTRead, d.RTTWrite)
	}
	if d.MonitorPubkey != "mon1" || d.ObservedAt != 1800 {
		t.Errorf("attribution = %q @ %d", d.MonitorPubkey, d.ObservedAt)
	}
}

// RTT fields default to -1 (not 0) so "not reported" is distinguishable from a
// genuine 0 ms — the browser can hide/last-sort unmeasured relays.
func TestParseRelayDiscoveryDefaultRTT(t *testing.T) {
	d := parseRelayDiscovery(&nostr.Event{
		Kind: 30166,
		Tags: [][]string{{"d", "wss://a.example"}},
	})
	if d == nil {
		t.Fatal("expected a relay")
	}
	if d.RTTOpen != -1 || d.RTTRead != -1 || d.RTTWrite != -1 {
		t.Errorf("unreported RTT should be -1, got %d/%d/%d", d.RTTOpen, d.RTTRead, d.RTTWrite)
	}
}
