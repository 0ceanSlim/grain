package nostrdb

import (
	"context"
	"strings"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"

	"encoding/hex"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// TestTagValuesRoundTrip exercises issue #65: tag values longer than 2
// chars are stored by nostrdb as offsets into the per-note strings table,
// and the reader struct ndb_str returns flag=0 for them (not NDB_PACKED_STR).
// An older noteToEventDirect whitelisted flag values STR/ID and silently
// dropped flag=0, so any tag value longer than 2 chars came back as "".
//
// This test publishes events covering each combination — short/long tag
// names, short/long values, single-letter and multi-letter tag names —
// and asserts the queried-back event has identical tags. If anyone
// re-introduces the whitelist regression, this test will fail with empty
// strings where actual values are expected.
func TestTagValuesRoundTrip(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()

	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	pub := hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey()))

	cases := []struct {
		name string
		kind int
		tags [][]string
	}{
		{
			// The exact case from issue #65: addressable kind 30100 with
			// a long d-tag value.
			name: "addressable_long_d_value",
			kind: 30100,
			tags: [][]string{{"d", "medecin-de-famille"}},
		},
		{
			// Multi-letter tag name (>2 chars) AND long value — both
			// stored as offsets, both lost in the regression.
			name: "long_name_and_value",
			kind: 30023,
			tags: [][]string{{"title", "A Reasonably Long Article Title"}, {"published_at", "1700000000"}},
		},
		{
			// Single-letter tag with a long value.
			name: "short_name_long_value",
			kind: 1,
			tags: [][]string{{"t", "this-is-a-very-long-hashtag-value"}},
		},
		{
			// Edge: exactly 3-char value (boundary between inline and
			// offset storage). Inline holds 3 bytes including a null
			// terminator, so 3-char strings still go to the table.
			name: "three_char_value",
			kind: 1,
			tags: [][]string{{"t", "abc"}},
		},
		{
			// Edge: 1-char value (inline path), should also continue to
			// work.
			name: "one_char_value",
			kind: 1,
			tags: [][]string{{"t", "x"}},
		},
		{
			// Mixed: a short single-letter tag, a long string tag, and
			// an event-id reference (PACKED_ID path) all in one event.
			name: "mixed_str_and_id",
			kind: 1,
			tags: [][]string{
				{"d", "ignored-on-kind-1-but-still-stored"},
				{"e", "0000000000000000000000000000000000000000000000000000000000000001"},
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Stagger created_at so addressable replacement doesn't
			// remove a prior case's event.
			ts := time.Now().Unix() + int64(len(tc.name))
			evt := signEvent(t, priv, pub, tc.kind, "tag round trip", tc.tags, ts)
			if err := db.StoreEvent(ctx, evt); err != nil {
				t.Fatalf("store: %v", err)
			}
			waitForIngest(t, db, evt.ID, true)

			txn, err := db.BeginQuery()
			if err != nil {
				t.Fatalf("begin query: %v", err)
			}
			got, err := txn.GetNoteByID(evt.ID)
			txn.EndQuery()
			if err != nil {
				t.Fatalf("get by id: %v", err)
			}
			if got == nil {
				t.Fatalf("event missing after ingest")
			}

			assertTagsEqual(t, tc.tags, got.Tags)
		})
	}
}

// assertTagsEqual compares expected and actual tag slices, with a clearer
// failure message than reflect.DeepEqual when issue #65 regresses (it
// shows precisely which values were emptied).
func assertTagsEqual(t *testing.T, want, got [][]string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("tag count mismatch: want %d, got %d (got=%v)", len(want), len(got), got)
	}
	for i := range want {
		if len(want[i]) != len(got[i]) {
			t.Fatalf("tag %d width mismatch: want %v, got %v", i, want[i], got[i])
		}
		for j := range want[i] {
			w, g := want[i][j], got[i][j]
			if w == g {
				continue
			}
			// Tag values that round-trip through nostrdb's id path
			// (32-byte hex like e/p tag values) come back hex-encoded;
			// the original is also hex, so they should match exactly.
			// If they don't, fail loudly. Empty got means issue #65.
			if g == "" {
				t.Fatalf("tag %d index %d emptied (issue #65 regression): want %q, got \"\"", i, j, w)
			}
			t.Fatalf("tag %d index %d differs: want %q, got %q", i, j, w, g)
		}
	}
}

// TestNoteToEventDirect_LongTagValue is a narrower assertion specifically
// targeting issue #65's exact reproduction: a kind 30100 with a single
// d-tag whose value is well past the 3-char inline boundary. If any code
// path drops the offset-string case again, this test fails with the
// canonical empty-string symptom.
func TestNoteToEventDirect_LongTagValue(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()

	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	pub := hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey()))

	const dValue = "medecin-de-famille"
	evt := signEvent(t, priv, pub, 30100, "{\"name\":\"Med\"}",
		[][]string{{"d", dValue}}, time.Now().Unix())

	if err := db.StoreEvent(ctx, evt); err != nil {
		t.Fatalf("store: %v", err)
	}
	waitForIngest(t, db, evt.ID, true)

	txn, err := db.BeginQuery()
	if err != nil {
		t.Fatalf("begin query: %v", err)
	}
	got, err := txn.GetNoteByID(evt.ID)
	txn.EndQuery()
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got == nil {
		t.Fatalf("event missing after ingest")
	}

	if len(got.Tags) != 1 || len(got.Tags[0]) != 2 {
		t.Fatalf("unexpected tag shape: %v", got.Tags)
	}
	if got.Tags[0][0] != "d" {
		t.Fatalf("tag name lost: got %q", got.Tags[0][0])
	}
	if got.Tags[0][1] != dValue {
		// The bug from issue #65 makes this empty. Surface the precise
		// symptom so future failures are diagnosed in seconds.
		if got.Tags[0][1] == "" {
			t.Fatalf("d-tag value emptied (issue #65 regression): want %q, got \"\"", dValue)
		}
		t.Fatalf("d-tag value differs: want %q, got %q", dValue, got.Tags[0][1])
	}

	// Also exercise the historical-query path (Query, not GetNoteByID),
	// since that's the path the bug reporter hit.
	txn, err = db.BeginQuery()
	if err != nil {
		t.Fatalf("begin query: %v", err)
	}
	results, err := txn.Query([]nostr.Filter{{Kinds: []int{30100}}}, 10)
	txn.EndQuery()
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("query returned no results")
	}
	for _, r := range results {
		if r.ID != evt.ID {
			continue
		}
		if len(r.Tags) != 1 || r.Tags[0][1] != dValue {
			t.Fatalf("query path d-tag value: want %q, got %q (full tags %v)",
				dValue, strings.Join([]string{r.Tags[0][1]}, ""), r.Tags)
		}
		return
	}
	t.Fatalf("event not in query results")
}
