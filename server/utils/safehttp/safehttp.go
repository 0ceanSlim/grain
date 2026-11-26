// Package safehttp is the HTTP client for fetches whose URL an outside party
// can influence (an admin-typed domain, a configured image URL): it connects
// only to public addresses, uses no proxy, and caps redirects.
package safehttp

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// blocked are non-public ranges the standard library's predicates miss.
var blocked = func() []netip.Prefix {
	var out []netip.Prefix
	for _, s := range []string{
		"0.0.0.0/8",      // "this network"
		"100.64.0.0/10",  // carrier-grade NAT, Tailscale
		"192.0.0.0/24",   // IETF protocol assignments
		"198.18.0.0/15",  // benchmarking
		"240.0.0.0/4",    // reserved, broadcast
		"64:ff9b::/96",   // NAT64 (wraps IPv4)
		"64:ff9b:1::/48", // local-use NAT64
		"2002::/16",      // 6to4 (wraps IPv4)
		"100::/64",       // discard
	} {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()

// AllowPrivate lets tests fetch from a local server; production never sets it.
var AllowPrivate = false

// IsPublic reports whether ip is a public unicast address.
func IsPublic(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() ||
		ip.IsInterfaceLocalMulticast() {
		return false
	}
	for _, p := range blocked {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// PublicOnly is a net.Dialer Control that refuses loopback, private,
// link-local and other non-public addresses. It runs on the resolved address
// at dial time, so a redirect or a DNS answer cannot point a fetch at the
// relay's own network.
func PublicOnly(_, address string, _ syscall.RawConn) error {
	if AllowPrivate {
		return nil
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("refusing to fetch from unparsable address %s", host)
	}
	if !IsPublic(ip) {
		return fmt.Errorf("refusing to fetch from non-public address %s", host)
	}
	return nil
}

// NewClient returns a client that dials public addresses only, through no
// proxy, within timeout, following at most maxRedirects http(s) redirects
// (0 refuses every redirect).
func NewClient(timeout time.Duration, maxRedirects int) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second, Control: PublicOnly}).DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
			MaxIdleConns:          4,
			IdleConnTimeout:       30 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > maxRedirects {
				return errors.New("redirects not followed")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to unsupported scheme %q", req.URL.Scheme)
			}
			return nil
		},
	}
}
