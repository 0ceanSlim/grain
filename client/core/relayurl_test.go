package core

import "testing"

func TestNormalizeRelayURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"wss://relay.example.com", "wss://relay.example.com", true},
		{"wss://relay.example.com/", "wss://relay.example.com", true},  // trailing slash
		{"relay.example.com", "wss://relay.example.com", true},         // bare host → wss
		{"WSS://Relay.Example.COM/", "wss://relay.example.com", true},  // case
		{"ws://localhost:8080", "ws://localhost:8080", true},           // ws preserved
		{"ws:/relay.example.com", "ws://relay.example.com", true},      // single-slash typo
		{"https://relay.example.com", "wss://relay.example.com", true}, // http(s)→ws(s)
		{"http://localhost:7777", "ws://localhost:7777", true},
		{"  wss://relay.example.com  ", "wss://relay.example.com", true},          // whitespace
		{"wss://relay.example.com/nostr/", "wss://relay.example.com/nostr", true}, // keep path
		{"", "", false},
		{"wss://", "", false}, // no host
	}
	for _, c := range cases {
		got, ok := normalizeRelayURL(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("normalizeRelayURL(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

// The same relay written three ways collapses to one.
func TestNormalizeRelayURLsDedupes(t *testing.T) {
	got := normalizeRelayURLs([]string{
		"wss://relay.example.com",
		"wss://relay.example.com/",
		"relay.example.com",
		"WSS://RELAY.EXAMPLE.COM",
		"", // dropped
		"ws://other.example.com",
	})
	if len(got) != 2 {
		t.Fatalf("want 2 distinct relays, got %v", got)
	}
	if got[0] != "wss://relay.example.com" || got[1] != "ws://other.example.com" {
		t.Fatalf("unexpected normalization/order: %v", got)
	}
}
