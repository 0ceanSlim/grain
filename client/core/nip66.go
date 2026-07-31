package core

import (
	"strconv"
	"strings"

	nostr "github.com/0ceanslim/grain/server/types"
)

// NIP-66 — Relay Discovery and Liveness Monitoring.
// https://github.com/nostr-protocol/nips/blob/master/66.md
//
// Two event kinds drive the self-discovering relay browser:
//   - kind 10166: a monitor's announcement — how often it checks and which
//     checks it runs. Discovering one is enough; a monitor publishes its 30166
//     set to relays where more monitors can be found, so the pool self-propagates.
//   - kind 30166: one addressable event per relay a monitor has probed, carrying
//     liveness + capability metadata (RTT, supported NIPs, network, requirements).
const (
	KindRelayMonitorAnnouncement = 10166
	KindRelayDiscovery           = 30166
)

// RelayMonitor is a NIP-66 monitor parsed from its kind-10166 announcement.
type RelayMonitor struct {
	Pubkey    string   // monitor identity (the announcement's author)
	Frequency int      // seconds between check rounds (0 = unspecified)
	Checks    []string // c tags: check types run (open, read, write, nip11, dns, geo, …)
	Geohash   string   // g tag (optional)
	CreatedAt int64    // announcement created_at
}

// parseMonitorAnnouncement maps a kind-10166 event to a RelayMonitor, or nil if
// the event isn't a monitor announcement.
func parseMonitorAnnouncement(ev *nostr.Event) *RelayMonitor {
	if ev == nil || ev.Kind != KindRelayMonitorAnnouncement {
		return nil
	}
	m := &RelayMonitor{Pubkey: ev.PubKey, CreatedAt: ev.CreatedAt}
	for _, tag := range ev.Tags {
		if len(tag) < 2 {
			continue
		}
		switch tag[0] {
		case "frequency":
			if n, err := strconv.Atoi(strings.TrimSpace(tag[1])); err == nil {
				m.Frequency = n
			}
		case "c":
			m.Checks = append(m.Checks, tag[1])
		case "g":
			if m.Geohash == "" {
				m.Geohash = tag[1]
			}
		}
	}
	return m
}

// DiscoveredRelay is one relay's liveness + capability snapshot parsed from a
// monitor's kind-30166 event. RTT fields are -1 when the monitor didn't report
// them. MonitorPubkey records the attribution so a later consensus pass
// (Phase 2) can require agreement across N monitors.
type DiscoveredRelay struct {
	URL           string   `json:"url"`                      // d tag: normalized ws(s):// URL
	Network       string   `json:"network,omitempty"`        // n tag: clearnet, tor, i2p, loki
	RelayType     string   `json:"relay_type,omitempty"`     // T tag: PascalCase relay type
	SupportedNIPs []int    `json:"supported_nips,omitempty"` // N tags
	Requirements  []string `json:"requirements,omitempty"`   // R tags: auth/writes/pow/payment, "!" = false
	Topics        []string `json:"topics,omitempty"`         // t tags
	AcceptedKinds []int    `json:"accepted_kinds,omitempty"` // k tags (non-negated only)
	Geohash       string   `json:"geohash,omitempty"`        // g tag (NIP-52)
	Language      string   `json:"language,omitempty"`       // l tag
	RTTOpen       int      `json:"rtt_open"`                 // rtt-open ms, -1 if absent
	RTTRead       int      `json:"rtt_read"`                 // rtt-read ms, -1 if absent
	RTTWrite      int      `json:"rtt_write"`                // rtt-write ms, -1 if absent
	MonitorPubkey string   `json:"monitor_pubkey"`           // which monitor reported this
	ObservedAt    int64    `json:"observed_at"`              // event created_at
}

// looksLikeRelayURL reports whether raw is shaped like a relay URL rather than a
// bare hex pubkey (the alternate NIP-66 d-tag form). Hex is 0-9a-f, so it can
// carry no ws/http scheme, dot, or colon — a URL always has at least one.
func looksLikeRelayURL(raw string) bool {
	s := strings.ToLower(strings.TrimSpace(raw))
	return strings.HasPrefix(s, "ws") || strings.HasPrefix(s, "http") ||
		strings.Contains(s, ".") || strings.Contains(s, ":")
}

// parseRelayDiscovery maps a kind-30166 event to a DiscoveredRelay, or nil if
// it isn't a 30166 or carries no usable relay URL. The d tag may be a hex
// pubkey rather than a URL (per NIP-66); those are skipped — the browser lists
// relay URLs.
func parseRelayDiscovery(ev *nostr.Event) *DiscoveredRelay {
	if ev == nil || ev.Kind != KindRelayDiscovery {
		return nil
	}
	d := &DiscoveredRelay{
		MonitorPubkey: ev.PubKey,
		ObservedAt:    ev.CreatedAt,
		RTTOpen:       -1,
		RTTRead:       -1,
		RTTWrite:      -1,
	}
	for _, tag := range ev.Tags {
		if len(tag) < 2 {
			continue
		}
		val := tag[1]
		switch tag[0] {
		case "d":
			// The d tag is a relay URL or (NIP-66's alternate form) a hex
			// pubkey. normalizeRelayURL is lenient enough to turn a bare pubkey
			// into "wss://<hex>", so gate on a URL-shaped value first.
			if looksLikeRelayURL(val) {
				if n, ok := normalizeRelayURL(val); ok {
					d.URL = n
				}
			}
		case "n":
			d.Network = val
		case "T":
			d.RelayType = val
		case "N":
			if nip, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
				d.SupportedNIPs = append(d.SupportedNIPs, nip)
			}
		case "R":
			d.Requirements = append(d.Requirements, val)
		case "t":
			d.Topics = append(d.Topics, val)
		case "k":
			// "!123" means the relay rejects kind 123; keep only accepted kinds.
			if !strings.HasPrefix(val, "!") {
				if k, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
					d.AcceptedKinds = append(d.AcceptedKinds, k)
				}
			}
		case "g":
			if d.Geohash == "" { // first (most precise) geohash
				d.Geohash = val
			}
		case "l":
			if d.Language == "" {
				d.Language = val
			}
		case "rtt-open":
			if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
				d.RTTOpen = n
			}
		case "rtt-read":
			if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
				d.RTTRead = n
			}
		case "rtt-write":
			if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
				d.RTTWrite = n
			}
		}
	}
	if d.URL == "" {
		return nil
	}
	return d
}
