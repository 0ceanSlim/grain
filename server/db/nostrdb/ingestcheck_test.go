package nostrdb

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// nostrdb's async ingester drops a note it can't parse or whose id it can't
// re-derive, after grain has already acked OK. Events holding &, < or >
// were lost that way for as long as grain fed it json.Marshal output.

func newTestKey(t *testing.T) (*btcec.PrivateKey, string) {
	t.Helper()
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	return priv, hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey()))
}

func TestStoreEvent_NIP01StringsRoundTrip(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()
	priv, pub := newTestKey(t)

	contents := []string{
		"a & b",
		"a < b > c",
		"https://x.com/p?a=1&b=2",
		"a &amp; b",
		`say "hi"`,
		`back\slash`,
		`literal & text`,
		"line\nbreak\r\ttab\bback\fform",
		"ctrl \x01 \x1f del \x7f",
		"sep    ",
		"café 日本語 🌾",
	}
	now := time.Now().Unix()
	for i, content := range contents {
		tags := [][]string{{"r", "https://x.com/?q=" + content}, {"t", content}}
		evt := signEvent(t, priv, pub, 1, content, tags, now-int64(i))
		if err := db.StoreEvent(ctx, evt); err != nil {
			t.Fatalf("store %q: %v", content, err)
		}
		waitForIngest(t, db, evt.ID, true)

		txn, err := db.BeginQuery()
		if err != nil {
			t.Fatalf("begin query: %v", err)
		}
		got, err := txn.GetNoteByID(evt.ID)
		txn.EndQuery()
		if err != nil || got == nil {
			t.Fatalf("get %q: %v", content, err)
		}
		if got.Content != content {
			t.Errorf("content: got %q, want %q", got.Content, content)
		}
		assertTagsEqual(t, tags, got.Tags)
	}
}

// A replacement nostrdb can't take must be refused before the old version
// is deleted. A NUL byte stops nostrdb's parser, so it stands in for any
// event the ingester would drop.
func TestStoreEvent_UnstorableReplacementKeepsOldVersion(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()
	priv, pub := newTestKey(t)
	now := time.Now().Unix()

	old := signEvent(t, priv, pub, 0, `{"name":"old"}`, nil, now-10)
	if err := db.StoreEvent(ctx, old); err != nil {
		t.Fatalf("store old: %v", err)
	}
	waitForIngest(t, db, old.ID, true)

	bad := signEvent(t, priv, pub, 0, "{\"name\":\"nul\x00\"}", nil, now)
	err := db.StoreEvent(ctx, bad)
	if err == nil || !strings.Contains(err.Error(), "nostrdb cannot store event") {
		t.Fatalf("store of unparseable replacement: got err %v, want a refusal", err)
	}

	// Give a wrongly queued delete time to land before checking.
	time.Sleep(300 * time.Millisecond)
	waitForIngest(t, db, old.ID, true)
}

func TestProcessDeletion_UnstorableDeletionDeletesNothing(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()
	priv, pub := newTestKey(t)
	now := time.Now().Unix()

	target := signEvent(t, priv, pub, 1, "keep me", nil, now-10)
	if err := db.StoreEvent(ctx, target); err != nil {
		t.Fatalf("store target: %v", err)
	}
	waitForIngest(t, db, target.ID, true)

	del := signEvent(t, priv, pub, 5, "reason\x00", [][]string{{"e", target.ID}}, now)
	if err := db.ProcessDeletion(ctx, del); err == nil {
		t.Fatal("unparseable deletion was accepted")
	}

	time.Sleep(300 * time.Millisecond)
	waitForIngest(t, db, target.ID, true)
}

// nostrdb packed any 64-char hex tag value into a binary id, uppercase too,
// and wrote it back lowercase: the recomputed id no longer matched, so the
// guard refused the event. Only lowercase hex is packed now.
func TestStoreEvent_UppercaseHexTagRoundTrips(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()
	priv, pub := newTestKey(t)

	upper := strings.ToUpper(strings.Repeat("ab", 32))
	tags := [][]string{{"x", upper}, {"e", strings.Repeat("cd", 32)}}
	evt := signEvent(t, priv, pub, 1, "uppercase hex tag", tags, time.Now().Unix())
	if err := db.StoreEvent(ctx, evt); err != nil {
		t.Fatalf("store: %v", err)
	}
	waitForIngest(t, db, evt.ID, true)

	txn, err := db.BeginQuery()
	if err != nil {
		t.Fatal(err)
	}
	got, err := txn.GetNoteByID(evt.ID)
	txn.EndQuery()
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	assertTagsEqual(t, tags, got.Tags)
}

// nostrdb's note parser read every string inside tags as a possible
// top-level key, so a tag spelled like an event field overwrote that field
// and the recomputed id no longer matched. Seen live as ["pubkey", <hex>].
func TestStoreEvent_TagStringsNamedLikeFields(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()
	priv, pub := newTestKey(t)
	hex64 := strings.Repeat("ab", 32)
	now := time.Now().Unix()

	for i, tags := range [][][]string{
		{{"pubkey", hex64}},
		{{"pubkey", "abc"}},
		{{"id", hex64}},
		{{"sig", hex64}},
		{{"content", "replaced?"}},
		{{"kind", "5"}},
		{{"created_at", "1"}},
		{{"tags", "x"}},
		{{"t", "content"}, {"t", "nostr"}},
		{{"t", "kind"}, {"t", "nostr"}},
		{{"t", "pubkey"}, {"p", hex64}},
	} {
		evt := signEvent(t, priv, pub, 1, "field-named tags", tags, now-int64(i))
		if err := db.StoreEvent(ctx, evt); err != nil {
			t.Errorf("tags %v: %v", tags, err)
			continue
		}
		waitForIngest(t, db, evt.ID, true)
		txn, err := db.BeginQuery()
		if err != nil {
			t.Fatal(err)
		}
		got, err := txn.GetNoteByID(evt.ID)
		txn.EndQuery()
		if err != nil || got == nil {
			t.Fatalf("tags %v: get: %v", tags, err)
		}
		if got.Content != evt.Content || got.PubKey != evt.PubKey || got.Kind != evt.Kind {
			t.Errorf("tags %v: fields changed: %+v", tags, got)
		}
		assertTagsEqual(t, tags, got.Tags)
	}
}
