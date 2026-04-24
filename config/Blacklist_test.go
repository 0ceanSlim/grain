package config

import (
	"testing"

	nostr "github.com/0ceanslim/grain/server/types"
)

func TestFirstTagValue(t *testing.T) {
	tags := [][]string{
		{"d", "mute"},
		{"p", "abc"},
		{"d", "second"}, // only the first match counts
	}
	if got := firstTagValue(tags, "d"); got != "mute" {
		t.Errorf("first d-tag: got %q, want %q", got, "mute")
	}
	if got := firstTagValue(tags, "missing"); got != "" {
		t.Errorf("missing tag: got %q, want empty string", got)
	}
	if got := firstTagValue(nil, "d"); got != "" {
		t.Errorf("nil tags: got %q, want empty string", got)
	}
	// Tag with only a key and no value is malformed per NIP-01 — skip.
	if got := firstTagValue([][]string{{"d"}}, "d"); got != "" {
		t.Errorf("malformed tag: got %q, want empty string", got)
	}
}

func TestLatestMuteListEventsPerKindD_ReplaceableSemantics(t *testing.T) {
	// Two kind:10000 events from the same author — only the latest should
	// win. This is the bug that made stale mute entries linger in the old
	// code, which accumulated pubkeys from every event it received.
	older := &nostr.Event{
		ID:        "older",
		Kind:      10000,
		CreatedAt: 100,
		Tags:      [][]string{{"p", "stale-muted"}},
	}
	newer := &nostr.Event{
		ID:        "newer",
		Kind:      10000,
		CreatedAt: 200,
		Tags:      [][]string{{"p", "fresh-muted"}},
	}

	got := latestMuteListEventsPerKindD([]*nostr.Event{older, newer})
	if len(got) != 1 {
		t.Fatalf("expected 1 winner per (kind,d), got %d", len(got))
	}
	if got[0].ID != "newer" {
		t.Errorf("expected newer event to win, got %q", got[0].ID)
	}
}

func TestLatestMuteListEventsPerKindD_AddressableGroupsByDTag(t *testing.T) {
	// Two different kind:30000 `d:"mute"` events should collapse to the
	// latest. An unrelated `d:"family"` should be dropped entirely since
	// only "mute" is a blacklist source.
	olderMute := &nostr.Event{ID: "older-mute", Kind: 30000, CreatedAt: 100, Tags: [][]string{{"d", "mute"}, {"p", "old"}}}
	newerMute := &nostr.Event{ID: "newer-mute", Kind: 30000, CreatedAt: 200, Tags: [][]string{{"d", "mute"}, {"p", "new"}}}
	family := &nostr.Event{ID: "family", Kind: 30000, CreatedAt: 300, Tags: [][]string{{"d", "family"}, {"p", "should-not-appear"}}}

	got := latestMuteListEventsPerKindD([]*nostr.Event{olderMute, newerMute, family})
	if len(got) != 1 {
		t.Fatalf("expected only d=mute to win, got %d winners", len(got))
	}
	if got[0].ID != "newer-mute" {
		t.Errorf("expected newer mute to win, got %q", got[0].ID)
	}
}

func TestLatestMuteListEventsPerKindD_DifferentKindsBothWin(t *testing.T) {
	// kind:10000 and kind:30000 d:"mute" both contribute — each is its own
	// bucket. The admin can configure both styles concurrently.
	kind10k := &nostr.Event{ID: "k10", Kind: 10000, CreatedAt: 100, Tags: [][]string{{"p", "via-10000"}}}
	kind30k := &nostr.Event{ID: "k30", Kind: 30000, CreatedAt: 50, Tags: [][]string{{"d", "mute"}, {"p", "via-30000"}}}

	got := latestMuteListEventsPerKindD([]*nostr.Event{kind10k, kind30k})
	if len(got) != 2 {
		t.Fatalf("expected both kinds to win, got %d", len(got))
	}
}

func TestLatestMuteListEventsPerKindD_IgnoresNil(t *testing.T) {
	valid := &nostr.Event{Kind: 10000, CreatedAt: 1, Tags: [][]string{{"p", "a"}}}
	got := latestMuteListEventsPerKindD([]*nostr.Event{nil, valid, nil})
	if len(got) != 1 {
		t.Fatalf("expected nil events to be skipped, got %d", len(got))
	}
}

func TestExtractMuteListPubkeys_DeduplicatesAcrossEvents(t *testing.T) {
	// Same pubkey listed in two winning events (e.g. kind:10000 + kind:30000
	// d:"mute") should only appear once in the result.
	a := &nostr.Event{Kind: 10000, Tags: [][]string{{"p", "dup"}, {"p", "unique-a"}}}
	b := &nostr.Event{Kind: 30000, Tags: [][]string{{"d", "mute"}, {"p", "dup"}, {"p", "unique-b"}}}

	got := extractMuteListPubkeys([]*nostr.Event{a, b}, "author")

	seen := map[string]int{}
	for _, pk := range got {
		seen[pk]++
	}
	if seen["dup"] != 1 {
		t.Errorf("expected duplicate pubkey to appear once, got %d", seen["dup"])
	}
	if seen["unique-a"] != 1 || seen["unique-b"] != 1 {
		t.Errorf("expected both unique pubkeys present, got %v", seen)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 deduped pubkeys, got %d (%v)", len(got), got)
	}
}

func TestExtractMuteListPubkeys_SkipsNonPTagsAndEmpty(t *testing.T) {
	ev := &nostr.Event{Kind: 10000, Tags: [][]string{
		{"p"},             // malformed — no value
		{"p", ""},         // empty value
		{"e", "event-id"}, // wrong tag
		{"p", "valid"},
	}}
	got := extractMuteListPubkeys([]*nostr.Event{ev}, "author")
	if len(got) != 1 || got[0] != "valid" {
		t.Errorf("expected [valid], got %v", got)
	}
}

func TestExtractMuteListPubkeys_HandlesEncryptedContent(t *testing.T) {
	// Event with encrypted content but public p tags — we take the public
	// entries and ignore the encrypted content (can't decrypt). Must not
	// crash on the non-empty content field.
	ev := &nostr.Event{
		Kind:    10000,
		Content: "not-really-ciphertext-but-non-empty",
		Tags:    [][]string{{"p", "public-mute"}},
	}
	got := extractMuteListPubkeys([]*nostr.Event{ev}, "author")
	if len(got) != 1 || got[0] != "public-mute" {
		t.Errorf("expected public pubkey extracted, got %v", got)
	}
}

func TestExtractMuteListPubkeys_IgnoresNil(t *testing.T) {
	got := extractMuteListPubkeys([]*nostr.Event{nil, nil}, "author")
	if len(got) != 0 {
		t.Errorf("expected empty result for nil events, got %v", got)
	}
}
