package branding

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"syscall"
	"time"
)

// FetchTimeout bounds one fetch of an image URL, connection to last byte. It
// runs inside the admin's NIP-86 request, so it stays under the server's
// write timeout (10 s by default) and the admin always gets an answer.
const FetchTimeout = 8 * time.Second

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

// allowPrivate lets tests fetch from a local server; production never sets it.
var allowPrivate = false

// publicOnly refuses connections to loopback, private, link-local and other
// non-public addresses. It runs on the resolved address at dial time, so a
// redirect or a DNS answer cannot point the fetch at the relay's own network.
func publicOnly(_, address string, _ syscall.RawConn) error {
	if allowPrivate {
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
	ip = ip.Unmap()
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() ||
		ip.IsInterfaceLocalMulticast() {
		return fmt.Errorf("refusing to fetch from non-public address %s", host)
	}
	for _, p := range blocked {
		if p.Contains(ip) {
			return fmt.Errorf("refusing to fetch from non-public address %s", host)
		}
	}
	return nil
}

var fetchClient = &http.Client{
	Timeout: FetchTimeout,
	Transport: &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, Control: publicOnly}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
		MaxIdleConns:          2,
		IdleConnTimeout:       30 * time.Second,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 3 {
			return errors.New("too many redirects")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return fmt.Errorf("redirect to unsupported scheme %q", req.URL.Scheme)
		}
		return nil
	},
}

// Fetch downloads an image URL once (http or https only, public addresses
// only, at most MaxImageBytes) and checks it.
func Fetch(ctx context.Context, rawURL string) (Image, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Image{}, fmt.Errorf("not an http(s) URL: %q", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Image{}, err
	}
	req.Header.Set("User-Agent", "grain (relay branding image fetch)")
	req.Header.Set("Accept", "image/png,image/webp,image/jpeg,image/gif,image/x-icon;q=0.9,*/*;q=0.1")
	resp, err := fetchClient.Do(req)
	if err != nil {
		return Image{}, fmt.Errorf("fetch %s: %w", u.Redacted(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Image{}, fmt.Errorf("fetch %s: HTTP %d", u.Redacted(), resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxImageBytes+1))
	if err != nil {
		return Image{}, fmt.Errorf("fetch %s: %w", u.Redacted(), err)
	}
	return Check(data)
}
