// Package relayurl applies NIP-42-flavored URL normalization for the
// purpose of comparing a client-supplied AUTH `relay` tag against the
// relay's configured Auth.RelayURL. NIP-42 says: "URL normalization
// techniques can be applied. For most cases just checking if the
// domain name is correct should be enough."
//
// Two match modes are supported, picked by the operator:
//
//   - ModeStrict (default): canonicalize scheme + host (lowercase,
//     strip default ports) and a trailing-slash-stripped path. Path is
//     significant. Safe for shared-host / multi-tenant deployments.
//
//   - ModeHost: drop the path entirely. Any AUTH addressed at the
//     right (canonicalized) scheme + host succeeds. Closer to the
//     "domain name is correct should be enough" reading. Use this if
//     clients in the wild append fingerprint suffixes to your URL and
//     you don't share a host with another relay.
package relayurl

import (
	"net/url"
	"strings"
)

// Mode controls how strictly URLs are compared. The zero value
// (ModeStrict) keeps path significant; ModeHost ignores it.
type Mode int

const (
	ModeStrict Mode = iota
	ModeHost
)

// ParseMode maps a config string ("", "strict", "host") to a Mode.
// Unknown values fall back to ModeStrict — fail-safe: stricter
// matching is the safer error.
func ParseMode(s string) Mode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "host":
		return ModeHost
	default:
		return ModeStrict
	}
}

// Match reports whether two relay URLs refer to the same relay under
// the given mode.
func Match(got, want string, mode Mode) bool {
	return canonical(got, mode) == canonical(want, mode)
}

// Canonical returns the strict-mode normalized form. Kept exported
// for callers (e.g. logs/diagnostics) that want a stable form.
func Canonical(s string) string {
	return canonical(s, ModeStrict)
}

func canonical(s string, mode Mode) string {
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		// Malformed input — fall back to a case-insensitive,
		// trailing-slash-trimmed string so a bad config doesn't
		// permanently break auth. In ModeHost we additionally drop
		// any path-looking suffix so the fallback respects the mode.
		out := strings.ToLower(strings.TrimRight(s, "/"))
		if mode == ModeHost {
			if i := strings.Index(out, "://"); i >= 0 {
				rest := out[i+3:]
				if j := strings.IndexAny(rest, "/?#"); j >= 0 {
					out = out[:i+3] + rest[:j]
				}
			} else if j := strings.IndexAny(out, "/?#"); j >= 0 {
				out = out[:j]
			}
		}
		return out
	}
	u.Scheme = strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Host)
	switch u.Scheme {
	case "wss", "https":
		host = strings.TrimSuffix(host, ":443")
	case "ws", "http":
		host = strings.TrimSuffix(host, ":80")
	}
	u.Host = host
	if mode == ModeHost {
		u.Path = ""
		u.RawPath = ""
	} else {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	// Drop user-info, query, and fragment — none are meaningful for
	// the relay-identity comparison NIP-42 describes.
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
