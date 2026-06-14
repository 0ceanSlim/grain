package core

import (
	"reflect"
	"testing"

	nostr "github.com/0ceanslim/grain/server/types"
)

// NIP-65 (10002): read+write is unmarked, read-only / write-only get the
// marker, old `r` tags are replaced, and non-relay tags survive.
func TestAssembleRelayListNIP65Markers(t *testing.T) {
	existing := &nostr.Event{
		PubKey:  "pk",
		Kind:    10002,
		Content: "keep",
		Tags: [][]string{
			{"r", "wss://old"},  // dropped — relay list is rewritten
			{"alt", "preserve"}, // preserved — not a relay tag
		},
	}
	entries := []RelayListEntry{
		{URL: "wss://a", Read: true, Write: true}, // both → unmarked
		{URL: "wss://b", Read: true},              // read-only
		{URL: "wss://c", Write: true},             // write-only
		{URL: "wss://a"},                          // duplicate — dropped
	}
	got, err := AssembleRelayListEvent(existing, 10002, "pk", entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Content != "keep" || got.Kind != 10002 || got.PubKey != "pk" {
		t.Fatalf("header not carried over: %+v", got)
	}
	if got.ID != "" || got.Sig != "" {
		t.Fatal("assembled event must be unsigned")
	}
	want := [][]string{
		{"alt", "preserve"},
		{"r", "wss://a"},
		{"r", "wss://b", "read"},
		{"r", "wss://c", "write"},
	}
	if !reflect.DeepEqual(got.Tags, want) {
		t.Fatalf("tags = %v, want %v", got.Tags, want)
	}
}

// The NIP-17 / NIP-51 list kinds use plain `relay` tags and ignore read/write.
func TestAssembleRelayListPlainRelayTags(t *testing.T) {
	for _, kind := range []int{10050, 10006, 10007, 10012} {
		got, err := AssembleRelayListEvent(nil, kind, "pk", []RelayListEntry{
			{URL: "wss://dm", Read: true}, // marker ignored for these kinds
			{URL: "wss://dm"},             // duplicate dropped
		})
		if err != nil {
			t.Fatalf("kind %d: unexpected error: %v", kind, err)
		}
		want := [][]string{{"relay", "wss://dm"}}
		if !reflect.DeepEqual(got.Tags, want) {
			t.Fatalf("kind %d tags = %v, want %v", kind, got.Tags, want)
		}
	}
}

func TestAssembleRelayListRejectsUnsupportedKind(t *testing.T) {
	if _, err := AssembleRelayListEvent(nil, 10063, "pk", nil); err == nil {
		t.Fatal("expected an error for a non relay-list kind (10063 is media)")
	}
}

func TestParseNIP65Entries(t *testing.T) {
	ev := &nostr.Event{Tags: [][]string{
		{"r", "wss://a"},          // both
		{"r", "wss://b", "read"},  // read-only
		{"r", "wss://c", "write"}, // write-only
		{"r", "wss://a", "write"}, // duplicate of a — flags OR together (stays both)
		{"other", "ignored"},
		{"r"}, // malformed
	}}
	got := ParseNIP65Entries(ev)
	want := []RelayListEntry{
		{URL: "wss://a", Read: true, Write: true},
		{URL: "wss://b", Read: true, Write: false},
		{URL: "wss://c", Read: false, Write: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseNIP65Entries = %v, want %v", got, want)
	}
}

func TestParseRelayTagURLs(t *testing.T) {
	ev := &nostr.Event{Tags: [][]string{
		{"relay", "wss://x"},
		{"relay", "wss://x"}, // duplicate dropped
		{"other", "y"},
		{"relay", "wss://y"},
		{"relay"}, // malformed
	}}
	got := ParseRelayTagURLs(ev)
	want := []string{"wss://x", "wss://y"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseRelayTagURLs = %v, want %v", got, want)
	}
}
