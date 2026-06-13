package core

import (
	"reflect"
	"testing"

	nostr "github.com/0ceanslim/grain/server/types"
)

func TestNormalizeMediaURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"blossom.band", "https://blossom.band", true},          // bare host → https
		{"https://blossom.band/", "https://blossom.band", true}, // trailing slash stripped
		{"https://Blossom.Band", "https://blossom.band", true},  // host lowercased
		{"http://example.com", "http://example.com", true},      // http kept
		{"https://nostr.build/upload/", "https://nostr.build/upload", true},
		{"wss://relay.example", "", false}, // ws/wss is not a media server
		{"  ", "", false},                  // blank
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := normalizeMediaURL(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("normalizeMediaURL(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseServerListEventOrderAndDedup(t *testing.T) {
	ev := &nostr.Event{Tags: [][]string{
		{"server", "https://blossom.band/"}, // normalises to https://blossom.band
		{"other", "ignored"},                // non-server tags skipped
		{"server", "https://nostr.build"},   //
		{"server", "https://blossom.band"},  // duplicate of the first after normalisation
		{"server"},                          // malformed (no value) skipped
		{"server", "wss://relay.example"},   // non-http skipped
	}}
	got := parseServerListEvent(ev)
	want := []string{"https://blossom.band", "https://nostr.build"}
	if len(got) != len(want) {
		t.Fatalf("parseServerListEvent = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseServerListEvent = %v, want %v (order matters: primary first)", got, want)
		}
	}
}

func TestLookupMediaServerInfo(t *testing.T) {
	if info, ok := LookupMediaServerInfo("https://blossom.band"); !ok || info.Kind != MediaKindBlossom {
		t.Errorf("LookupMediaServerInfo(blossom.band) = (%+v, %v), want a blossom hit", info, ok)
	}
	// Matches regardless of input normalisation.
	if _, ok := LookupMediaServerInfo("blossom.band"); !ok {
		t.Error("LookupMediaServerInfo should match an un-normalised URL")
	}
	if _, ok := LookupMediaServerInfo("https://unknown.example"); ok {
		t.Error("LookupMediaServerInfo should miss an unknown server")
	}
}

func TestMediaServersHasAny(t *testing.T) {
	if (&MediaServers{}).HasAny() {
		t.Error("empty MediaServers should report no servers")
	}
	if !(&MediaServers{Blossom: []string{"https://x"}}).HasAny() {
		t.Error("MediaServers with a Blossom server should report HasAny")
	}
	if (*MediaServers)(nil).HasAny() {
		t.Error("nil MediaServers should report no servers")
	}
}

// The curated suggestions back the settings UI and the upload picker, so the
// table must stay self-consistent: every URL already normalised, valid kinds /
// costs / retentions, and no NIP-96 server ever flagged as a mirror target
// (BUD-04 is Blossom-only).
func TestSuggestedMediaServersAreWellFormed(t *testing.T) {
	for _, info := range SuggestedMediaServers() {
		if norm, ok := normalizeMediaURL(info.URL); !ok || norm != info.URL {
			t.Errorf("suggested %q is not already normalised (got %q)", info.URL, norm)
		}
		switch info.Kind {
		case MediaKindBlossom, MediaKindNIP96:
		default:
			t.Errorf("suggested %q has invalid kind %q", info.URL, info.Kind)
		}
		switch info.Cost {
		case "free", "paid":
		default:
			t.Errorf("suggested %q has invalid cost %q", info.URL, info.Cost)
		}
		switch info.Retention {
		case "permanent", "ephemeral":
		default:
			t.Errorf("suggested %q has invalid retention %q", info.URL, info.Retention)
		}
		if info.Kind == MediaKindNIP96 && info.Mirror {
			t.Errorf("suggested %q is NIP-96 but flagged as a mirror target (BUD-04 is Blossom-only)", info.URL)
		}
	}
}

// Saving a list must rewrite the server tags in order while preserving every
// other tag and the content — the same "don't drop data" rule as the kind-0
// editor.
func TestAssembleMediaServerEventPreservesOtherTags(t *testing.T) {
	existing := &nostr.Event{
		PubKey:  "abc",
		Kind:    10063,
		Content: "keep me",
		Tags: [][]string{
			{"server", "https://old.example"}, // dropped — the server list is rewritten
			{"client", "amethyst"},            // preserved — not a server tag
		},
	}
	got, err := AssembleMediaServerEvent(existing, 10063, "abc",
		[]string{"https://blossom.band/", "https://blossom.band", "https://nostr.build"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != 10063 || got.PubKey != "abc" || got.Content != "keep me" {
		t.Fatalf("kind/pubkey/content not carried over: %+v", got)
	}
	if got.ID != "" || got.Sig != "" {
		t.Fatal("assembled event must be unsigned")
	}
	want := [][]string{
		{"client", "amethyst"},
		{"server", "https://blossom.band"}, // normalised + deduped, order preserved
		{"server", "https://nostr.build"},
	}
	if !reflect.DeepEqual(got.Tags, want) {
		t.Fatalf("tags = %v, want %v", got.Tags, want)
	}
}

func TestAssembleMediaServerEventNilExistingAndBadKind(t *testing.T) {
	got, err := AssembleMediaServerEvent(nil, 10096, "pk", []string{"nostr.build"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != 10096 || got.PubKey != "pk" || len(got.Tags) != 1 || got.Tags[0][1] != "https://nostr.build" {
		t.Fatalf("nil-existing build wrong: %+v", got)
	}
	if _, err := AssembleMediaServerEvent(nil, 10002, "pk", nil); err == nil {
		t.Fatal("expected an error for an unsupported kind")
	}
}
