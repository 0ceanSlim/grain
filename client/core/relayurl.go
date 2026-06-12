package core

import (
	"net/url"
	"strings"
)

// normalizeRelayURL canonicalises a relay URL for deduplication. It repairs and
// normalises the scheme (bare hosts default to wss; http(s)→ws(s); single-slash
// typos like "ws:/host" are fixed), lowercases the host, and strips a trailing
// slash. ws and wss are kept distinct — they are different transports, not typos
// of each other. Returns ("", false) for input with no usable host.
//
// Relay lists in the wild are messy (mistyped schemes, trailing slashes, mixed
// case), so every relay URL entering the directory or the pool goes through here
// to keep the "known" / connection sets free of near-duplicates.
func normalizeRelayURL(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}

	scheme := "wss"
	lower := strings.ToLower(s)
	// Longest / most-specific prefixes first so "wss://" wins over "wss:".
	for _, p := range []struct{ prefix, sch string }{
		{"wss://", "wss"}, {"ws://", "ws"},
		{"https://", "wss"}, {"http://", "ws"},
		{"wss:/", "wss"}, {"ws:/", "ws"},
		{"wss:", "wss"}, {"ws:", "ws"},
	} {
		if strings.HasPrefix(lower, p.prefix) {
			scheme = p.sch
			s = s[len(p.prefix):]
			break
		}
	}
	s = strings.TrimLeft(s, "/") // drop any stray leading slashes left by a typo

	u, err := url.Parse(scheme + "://" + s)
	if err != nil || u.Host == "" {
		return "", false
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	u.Fragment = ""
	return u.String(), true
}

// normalizeRelayURLs normalises a list, dropping invalid entries and duplicates
// while preserving first-seen order.
func normalizeRelayURLs(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		n, ok := normalizeRelayURL(r)
		if !ok {
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}
