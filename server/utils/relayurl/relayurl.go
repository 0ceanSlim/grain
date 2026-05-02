// Package relayurl applies NIP-42-flavored URL normalization for the
// purpose of comparing a client-supplied AUTH `relay` tag against the
// relay's configured Auth.RelayURL. NIP-42 says: "URL normalization
// techniques can be applied. For most cases just checking if the
// domain name is correct should be enough."
//
// We canonicalize scheme + host (lowercase), strip default ports
// (443 for wss/https, 80 for ws/http), and strip a single trailing
// slash from the path. We don't go all the way to "domain only" so
// a relay running on a non-standard path or shared host can't
// accept AUTH addressed elsewhere.
package relayurl

import (
	"net/url"
	"strings"
)

// Match reports whether two relay URLs refer to the same relay
// after canonicalization.
func Match(got, want string) bool {
	return Canonical(got) == Canonical(want)
}

// Canonical returns a normalized form of s suitable for equality
// comparison. If s fails to parse as a URL we fall back to a
// case-insensitive trailing-slash-trimmed string so a malformed
// config doesn't permanently break auth.
func Canonical(s string) string {
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return strings.ToLower(strings.TrimRight(s, "/"))
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
	u.Path = strings.TrimRight(u.Path, "/")
	// Drop user-info, query, and fragment — none are meaningful for
	// the relay-identity comparison NIP-42 describes.
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
