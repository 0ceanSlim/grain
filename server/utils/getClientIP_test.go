package utils

import (
	"net/http/httptest"
	"testing"
)

func TestGetClientIP(t *testing.T) {
	t.Cleanup(func() { TrustedProxiesProvider = nil })
	for _, c := range []struct {
		name, peer, xff string
		proxies         []string
		want            string
	}{
		{"public peer, header ignored", "198.51.100.7:4444", "203.0.113.9", nil, "198.51.100.7"},
		{"public peer, blank first token", "198.51.100.7:4444", ", 203.0.113.9", nil, "198.51.100.7"},
		{"no header", "198.51.100.7:4444", "", nil, "198.51.100.7"},
		{"local proxy", "127.0.0.1:5000", "203.0.113.9", nil, "203.0.113.9"},
		{"local proxy, forged left entries", "127.0.0.1:5000", "10.9.8.7, 1.2.3.4, 203.0.113.5", nil, "203.0.113.5"},
		{"proxy chain", "10.0.0.2:5000", "203.0.113.5, 10.0.0.1", nil, "203.0.113.5"},
		{"local proxy, blank header", "127.0.0.1:5000", ",", nil, "127.0.0.1"},
		{"local proxy, garbage", "127.0.0.1:5000", "<x>, 203.0.113.5", nil, "203.0.113.5"},
		{"local proxy, garbage nearest", "127.0.0.1:5000", "203.0.113.5, <x>", nil, "127.0.0.1"},
		{"trust none", "127.0.0.1:5000", "203.0.113.9", []string{}, "127.0.0.1"},
		{"configured proxy", "198.51.100.7:4444", "203.0.113.9", []string{"198.51.100.0/24"}, "203.0.113.9"},
		{"ipv6 peer", "[2001:db8::1]:443", "203.0.113.9", nil, "2001:db8::1"},
		{"mapped ipv4 peer", "[::ffff:127.0.0.1]:5000", "203.0.113.9", nil, "203.0.113.9"},
	} {
		proxies := c.proxies
		TrustedProxiesProvider = func() []string { return proxies }
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.peer
		if c.xff != "" {
			r.Header.Set("X-Forwarded-For", c.xff)
		}
		if got := GetClientIP(r); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestParseTrustedProxies(t *testing.T) {
	if _, err := ParseTrustedProxies([]string{"10.0.0.0/8", "203.0.113.4", "::1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseTrustedProxies([]string{"proxy.example"}); err == nil {
		t.Fatal("a host name was accepted")
	}
}
