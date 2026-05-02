package relayurl

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
		ok   bool
	}{
		{"identical", "wss://relay.example.com", "wss://relay.example.com", true},
		{"trailing slash on client", "wss://relay.example.com/", "wss://relay.example.com", true},
		{"trailing slash on config", "wss://relay.example.com", "wss://relay.example.com/", true},
		{"trailing slash both sides", "wss://relay.example.com/", "wss://relay.example.com/", true},
		{"explicit default port wss", "wss://relay.example.com:443", "wss://relay.example.com", true},
		{"explicit default port ws", "ws://relay.example.com:80", "ws://relay.example.com", true},
		{"upper-case host", "wss://Relay.Example.COM", "wss://relay.example.com", true},
		{"different host", "wss://other.example.com", "wss://relay.example.com", false},
		{"different scheme", "ws://relay.example.com", "wss://relay.example.com", false},
		{"non-default port not stripped", "wss://relay.example.com:8443", "wss://relay.example.com", false},
		{"different path rejected", "wss://relay.example.com/v2", "wss://relay.example.com", false},
		{"path trailing slash equiv", "wss://relay.example.com/v2/", "wss://relay.example.com/v2", true},
		{"empty got", "", "wss://relay.example.com", false},
		{"empty config", "wss://relay.example.com", "", false},
		{"both empty", "", "", true},
		{"query stripped", "wss://relay.example.com/?x=1", "wss://relay.example.com", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Match(tc.got, tc.want); got != tc.ok {
				t.Errorf("Match(%q, %q) = %v, want %v", tc.got, tc.want, got, tc.ok)
			}
		})
	}
}
