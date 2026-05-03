package relayurl

import "testing"

func TestParseMode(t *testing.T) {
	cases := []struct {
		in   string
		want Mode
	}{
		{"", ModeStrict},
		{"strict", ModeStrict},
		{"STRICT", ModeStrict},
		{"  strict  ", ModeStrict},
		{"host", ModeHost},
		{"HOST", ModeHost},
		{"unknown-value", ModeStrict}, // fail-safe to strict
	}
	for _, tc := range cases {
		if got := ParseMode(tc.in); got != tc.want {
			t.Errorf("ParseMode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestMatchStrict(t *testing.T) {
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
		{"path suffix from misbehaving client", "wss://relay.example.com/marble-november-sable", "wss://relay.example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Match(tc.got, tc.want, ModeStrict); got != tc.ok {
				t.Errorf("Match(%q, %q, strict) = %v, want %v", tc.got, tc.want, got, tc.ok)
			}
		})
	}
}

func TestMatchHost(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
		ok   bool
	}{
		{"identical", "wss://relay.example.com", "wss://relay.example.com", true},
		{"trailing slash", "wss://relay.example.com/", "wss://relay.example.com", true},
		{"upper-case host", "wss://Relay.Example.COM", "wss://relay.example.com", true},
		{"port stripped", "wss://relay.example.com:443", "wss://relay.example.com", true},

		// the whole point of host mode: arbitrary path suffix accepted
		{"path suffix accepted", "wss://relay.example.com/marble-november-sable", "wss://relay.example.com", true},
		{"deep path accepted", "wss://relay.example.com/a/b/c", "wss://relay.example.com", true},
		{"query+fragment accepted", "wss://relay.example.com/x?y=1#z", "wss://relay.example.com", true},

		// scheme and host are still significant
		{"different scheme rejected", "ws://relay.example.com", "wss://relay.example.com", false},
		{"different host rejected", "wss://other.example.com", "wss://relay.example.com", false},
		{"different non-default port rejected", "wss://relay.example.com:8443", "wss://relay.example.com", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Match(tc.got, tc.want, ModeHost); got != tc.ok {
				t.Errorf("Match(%q, %q, host) = %v, want %v", tc.got, tc.want, got, tc.ok)
			}
		})
	}
}
