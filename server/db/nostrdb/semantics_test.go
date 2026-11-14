package nostrdb

import (
	"context"
	"strings"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// storeAll stores events and fails the test on the first error.
func storeAll(t *testing.T, db *NDB, evts ...nostr.Event) {
	t.Helper()
	for _, evt := range evts {
		if err := db.StoreEvent(context.Background(), evt); err != nil {
			t.Fatalf("store %s: %v", evt.ID, err)
		}
	}
}

// NIP-01: since <= created_at <= until, whichever index the query uses.
func TestQuery_UntilIsInclusive(t *testing.T) {
	db := openTempDB(t)
	priv, pub := newTestKey(t)
	ts := time.Now().Unix() - 100

	evt := signEvent(t, priv, pub, 5000, "at until", [][]string{{"t", "inclusive"}}, ts)
	storeAll(t, db, evt)

	at := time.Unix(ts, 0)
	limit := 10
	for name, f := range map[string]nostr.Filter{
		"authors+kinds": {Authors: []string{pub}, Kinds: []int{5000}},
		"kinds":         {Kinds: []int{5000}},
		"authors":       {Authors: []string{pub}},
		"ids":           {IDs: []string{evt.ID}},
		"tags":          {Tags: map[string][]string{"t": {"inclusive"}}},
		"no index":      {},
	} {
		f.Until, f.Since, f.Limit = &at, &at, &limit
		got, err := db.Query([]nostr.Filter{f}, limit)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(got) != 1 || got[0].ID != evt.ID {
			t.Errorf("%s: an event at exactly since = until wasn't returned (got %d)", name, len(got))
		}
	}
}

// NIP-01: limit 0 asks for no stored events.
func TestQuery_LimitZero(t *testing.T) {
	db := openTempDB(t)
	priv, pub := newTestKey(t)
	storeAll(t, db, signEvent(t, priv, pub, 1, "stored", nil, time.Now().Unix()))

	zero := 0
	got, err := db.Query([]nostr.Filter{{Authors: []string{pub}, Limit: &zero}}, 100)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("limit 0 returned %d stored events", len(got))
	}
}

// COUNT stays exact when a second's events straddle a page boundary, and
// says approximate when one second holds more than a page.
func TestCount_PageBoundaries(t *testing.T) {
	db := openTempDB(t)
	old := countPageSize
	countPageSize = 10
	t.Cleanup(func() { countPageSize = old })

	priv, pub := newTestKey(t)
	base := time.Now().Unix() - 1000

	// 7 seconds of 3 events each: every page of 10 ends mid-second.
	want := 0
	for sec := int64(0); sec < 7; sec++ {
		for i := 0; i < 3; i++ {
			storeAll(t, db, signEvent(t, priv, pub, 1, "count", [][]string{{"n", string(rune('a' + i))}}, base+sec))
			want++
		}
	}
	got, approx, err := db.CountFiltered([]nostr.Filter{{Authors: []string{pub}}})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if got != want || approx {
		t.Fatalf("count = %d (approximate %v), want exactly %d", got, approx, want)
	}

	// 12 events in one second can't fit a page of 10.
	priv2, pub2 := newTestKey(t)
	for i := 0; i < 12; i++ {
		storeAll(t, db, signEvent(t, priv2, pub2, 1, strings.Repeat("x", i+1), nil, base))
	}
	_, approx, err = db.CountFiltered([]nostr.Filter{{Authors: []string{pub2}}})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if !approx {
		t.Fatal("a second larger than a page was counted as exact")
	}
}

// A deleted event can't be published again, and neither can a version of
// a deleted address older than its deletion.
func TestTombstones(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()
	priv, pub := newTestKey(t)
	now := time.Now().Unix()

	note := signEvent(t, priv, pub, 1, "delete me", nil, now-10)
	storeAll(t, db, note)
	if err := db.ProcessDeletion(ctx, signEvent(t, priv, pub, 5, "", [][]string{{"e", note.ID}}, now-5)); err != nil {
		t.Fatalf("delete by id: %v", err)
	}
	if err := db.StoreEvent(ctx, note); err == nil || !strings.HasPrefix(err.Error(), "blocked:") {
		t.Fatalf("a deleted event was accepted again: %v", err)
	}

	// Someone else's deletion request doesn't count.
	otherPriv, otherPub := newTestKey(t)
	other := signEvent(t, priv, pub, 1, "not theirs to delete", nil, now-10)
	if err := db.ProcessDeletion(ctx, signEvent(t, otherPriv, otherPub, 5, "", [][]string{{"e", other.ID}}, now-5)); err != nil {
		t.Fatalf("foreign deletion: %v", err)
	}
	storeAll(t, db, other)

	addr := "30023:" + pub + ":post"
	article := signEvent(t, priv, pub, 30023, "v1", [][]string{{"d", "post"}}, now-20)
	storeAll(t, db, article)
	if err := db.ProcessDeletion(ctx, signEvent(t, priv, pub, 5, "", [][]string{{"a", addr}}, now-15)); err != nil {
		t.Fatalf("delete by address: %v", err)
	}
	older := signEvent(t, priv, pub, 30023, "v0", [][]string{{"d", "post"}}, now-18)
	if err := db.StoreEvent(ctx, older); err == nil || !strings.HasPrefix(err.Error(), "blocked:") {
		t.Fatalf("a version older than the address's deletion was accepted: %v", err)
	}
	newer := signEvent(t, priv, pub, 30023, "v2", [][]string{{"d", "post"}}, now-1)
	storeAll(t, db, newer)
}
