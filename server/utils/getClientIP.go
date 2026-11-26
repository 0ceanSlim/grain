package utils

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
)

// DefaultTrustedProxies are the peers whose X-Forwarded-For grain believes
// when server.trusted_proxies is unset: loopback and private ranges, where a
// reverse proxy on the same host, in Docker or on a platform's private
// network connects from. A relay reached directly from the internet sees
// public peers, so their header is ignored.
var DefaultTrustedProxies = []string{
	"127.0.0.0/8", "::1/128",
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16",
	"fc00::/7",
}

// TrustedProxiesProvider returns the configured proxy list (startup wires it
// to server.trusted_proxies): nil means DefaultTrustedProxies, an empty list
// trusts no proxy.
var TrustedProxiesProvider func() []string

// ParseTrustedProxies parses CIDRs or bare addresses.
func ParseTrustedProxies(list []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(list))
	for _, s := range list {
		s = strings.TrimSpace(s)
		if p, err := netip.ParsePrefix(s); err == nil {
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(s)
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q is not an address or CIDR", s)
		}
		out = append(out, netip.PrefixFrom(a.Unmap(), a.Unmap().BitLen()))
	}
	return out, nil
}

var proxyCache struct {
	sync.Mutex
	key      string
	prefixes []netip.Prefix
}

func trustedProxies() []netip.Prefix {
	list := DefaultTrustedProxies
	if TrustedProxiesProvider != nil {
		if l := TrustedProxiesProvider(); l != nil {
			list = l
		}
	}
	key := strings.Join(list, ",")
	proxyCache.Lock()
	defer proxyCache.Unlock()
	if proxyCache.prefixes == nil || proxyCache.key != key {
		p, err := ParseTrustedProxies(list)
		if err != nil {
			p = nil // config validation rejects this; trust nothing meanwhile
		}
		proxyCache.key, proxyCache.prefixes = key, append([]netip.Prefix{}, p...)
	}
	return proxyCache.prefixes
}

func isTrusted(ip netip.Addr, proxies []netip.Prefix) bool {
	ip = ip.Unmap()
	for _, p := range proxies {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// GetClientIP returns the address the request came from: the TCP peer, or,
// when the peer is a trusted proxy, the nearest address in X-Forwarded-For
// that isn't one. The rate limiter and the IP block list key on it, so a
// client can't choose it, and it is never empty.
func GetClientIP(r *http.Request) string {
	peer := r.RemoteAddr
	if host, _, err := net.SplitHostPort(peer); err == nil {
		peer = host
	}
	peerIP, err := netip.ParseAddr(peer)
	if err != nil {
		return peer
	}
	proxies := trustedProxies()
	if !isTrusted(peerIP, proxies) {
		return peerIP.Unmap().String()
	}

	// Walk the chain from the nearest hop back. Each trusted proxy appended
	// the address it received from; the first untrusted one is the client.
	client := peerIP.Unmap()
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(hops[i])
		if hop == "" {
			continue
		}
		ip, err := netip.ParseAddr(hop)
		if err != nil {
			if host, _, err2 := net.SplitHostPort(hop); err2 == nil {
				ip, err = netip.ParseAddr(host)
			}
		}
		if err != nil {
			break // garbage from here back was written by the client
		}
		client = ip.Unmap()
		if !isTrusted(client, proxies) {
			break
		}
	}
	return client.String()
}
