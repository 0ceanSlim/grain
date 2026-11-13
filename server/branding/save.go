package branding

import (
	"net/http"
	"net/url"
	"strings"
)

// PublicBase is the relay's public web origin ("https://relay.example"). It
// comes from auth.relay_url when set (the operator's declared public URL;
// ws/wss map to http/https), else from the request. "" when neither says.
// Used to make a stored image's relative path absolute where a client on
// another origin needs it (the NIP-11 icon and banner), at serve time, so a
// domain move or an admin session on localhost never bakes a wrong origin in.
func PublicBase(relayURL string, r *http.Request) string {
	if u, err := url.Parse(strings.TrimSpace(relayURL)); err == nil && u.Host != "" {
		switch u.Scheme {
		case "wss", "https":
			return "https://" + u.Host
		case "ws", "http":
			return "http://" + u.Host
		}
	}
	if r != nil && r.Host != "" {
		scheme := "http"
		if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		return scheme + "://" + r.Host
	}
	return ""
}

// Absolute makes a relay-relative path ("/branding/...", "/static/...")
// absolute on base; anything else (already absolute, empty, or no base) is
// returned unchanged.
func Absolute(v, base string) string {
	if base == "" || !IsLocalPath(v) {
		return v
	}
	return strings.TrimRight(base, "/") + v
}

// IsLocalPath reports whether v is a plain path on this relay ("/x"): not
// "//x" or "/\x" (which browsers read as another host), and no scheme.
func IsLocalPath(v string) bool {
	if !strings.HasPrefix(v, "/") || strings.HasPrefix(v, "//") || strings.ContainsAny(v, "\\\r\n\t") {
		return false
	}
	u, err := url.Parse(v)
	return err == nil && u.Scheme == "" && u.Host == ""
}

// Save stores img for slot and points the slot's relay metadata field at the
// self-hosted copy (the relay-relative path /branding/<file>) through
// setField (utils.SetRelayMetadataField). Returns the stored path. The
// caller prunes afterwards (Prune, with every branding field), holding one
// lock across both so a prune never removes a file another store has
// written but not yet pointed its field at.
func Save(slot string, img Image, setField func(field, value string) error) (string, error) {
	name, err := Store(slot, img)
	if err != nil {
		return "", err
	}
	p := URLPrefix + name
	if err := setField(slot, p); err != nil {
		return "", err
	}
	return p, nil
}
