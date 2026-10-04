package nostrdb

import (
	"context"
	"strings"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"

	"crypto/sha256"
	"encoding/hex"
)

// signEvent is a tiny local helper that mirrors tests/helpers.go so this
// package-level test doesn't pull in the top-level tests/ helpers.
func signEvent(t *testing.T, priv *btcec.PrivateKey, pub string, kind int, content string, tags [][]string, ts int64) nostr.Event {
	t.Helper()
	if tags == nil {
		tags = [][]string{}
	}
	evt := nostr.Event{
		PubKey:    pub,
		CreatedAt: ts,
		Kind:      kind,
		Tags:      tags,
		Content:   content,
	}
	h := sha256.Sum256(evt.Commitment())
	evt.ID = hex.EncodeToString(h[:])
	sig, err := schnorr.Sign(priv, h[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	evt.Sig = hex.EncodeToString(sig.Serialize())
	return evt
}

// openTempDB opens a fresh nostrdb in a test-owned temp directory and
// registers cleanup.
func openTempDB(t *testing.T) *NDB {
	t.Helper()
	dir := t.TempDir()
	db, err := Open(dir, 32, 1)
	if err != nil {
		t.Fatalf("open nostrdb: %v", err)
	}
	t.Cleanup(db.Close)
	return db
}

// waitForIngest polls until an event is queryable, or fails the test. The
// nostrdb writer thread is asynchronous; ingest doesn't become visible to
// the reader txn until the writer batch commits.
func waitForIngest(t *testing.T, db *NDB, id string, present bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		txn, err := db.BeginQuery()
		if err != nil {
			t.Fatalf("begin query: %v", err)
		}
		got, err := txn.GetNoteByID(id)
		txn.EndQuery()
		if err != nil {
			t.Fatalf("get by id: %v", err)
		}
		if (got != nil) == present {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for event %s present=%v", id, present)
}

func TestDeleteNoteByID_RoundTrip(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()

	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	pub := hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey()))

	evt := signEvent(t, priv, pub, 1, "round-trip delete", nil, time.Now().Unix())
	if err := db.StoreEvent(ctx, evt); err != nil {
		t.Fatalf("store: %v", err)
	}
	waitForIngest(t, db, evt.ID, true)

	idBytes, err := hexToBytes32(evt.ID)
	if err != nil {
		t.Fatalf("decode id: %v", err)
	}
	var id32 [32]byte
	copy(id32[:], idBytes)
	if err := db.DeleteNoteByID(id32); err != nil {
		t.Fatalf("delete: %v", err)
	}
	waitForIngest(t, db, evt.ID, false)
}

func TestDeleteNoteByID_NotFound(t *testing.T) {
	db := openTempDB(t)
	var id32 [32]byte
	for i := range id32 {
		id32[i] = 0xAB
	}
	// No event at this id — delete should enqueue cleanly (the C side is
	// a silent no-op on missing ids).
	if err := db.DeleteNoteByID(id32); err != nil {
		t.Fatalf("delete of missing id returned error: %v", err)
	}
}

func TestDeleteNoteByID_ReingestAfterDelete(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()

	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	pub := hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey()))

	evt := signEvent(t, priv, pub, 1, "ghost", nil, time.Now().Unix())
	if err := db.StoreEvent(ctx, evt); err != nil {
		t.Fatalf("store: %v", err)
	}
	waitForIngest(t, db, evt.ID, true)

	idBytes, _ := hexToBytes32(evt.ID)
	var id32 [32]byte
	copy(id32[:], idBytes)
	if err := db.DeleteNoteByID(id32); err != nil {
		t.Fatalf("delete: %v", err)
	}
	waitForIngest(t, db, evt.ID, false)

	// After delete, the same event must be ingestable again — ie the
	// duplicate-id check in nostrdb no longer sees it, and no sub-index
	// entry lingers.
	if err := db.StoreEvent(ctx, evt); err != nil {
		t.Fatalf("re-store: %v", err)
	}
	waitForIngest(t, db, evt.ID, true)
}

// A deletion request with a malformed target is refused with "invalid:" and
// deletes nothing, even beside a valid target. Another author's event is
// ignored without an error.
func TestProcessDeletion_Targets(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()
	priv, pub := newTestKey(t)
	otherPriv, otherPub := newTestKey(t)
	now := time.Now().Unix()

	mine := signEvent(t, priv, pub, 1, "mine", nil, now-10)
	theirs := signEvent(t, otherPriv, otherPub, 1, "theirs", nil, now-10)
	for _, evt := range []nostr.Event{mine, theirs} {
		if err := db.StoreEvent(ctx, evt); err != nil {
			t.Fatal(err)
		}
		waitForIngest(t, db, evt.ID, true)
	}

	for _, tags := range [][][]string{
		{{"e", ""}, {"e", mine.ID}},
		{{"e", mine.ID}, {"e", "nothex"}},
		{{"e"}},
		{{"a", "30023:" + pub}},
		{{"a", "x:" + pub + ":d"}},
		{{"a", "30023:nothex:d"}},
	} {
		del := signEvent(t, priv, pub, 5, "", tags, now)
		err := db.ProcessDeletion(ctx, del)
		if err == nil || !strings.HasPrefix(err.Error(), "invalid: ") {
			t.Errorf("tags %v: err = %v, want an invalid: refusal", tags, err)
		}
	}
	time.Sleep(200 * time.Millisecond)
	waitForIngest(t, db, mine.ID, true)

	del := signEvent(t, priv, pub, 5, "", [][]string{{"e", theirs.ID}, {"a", "30023:" + otherPub + ":d"}}, now)
	if err := db.ProcessDeletion(ctx, del); err != nil {
		t.Fatalf("deletion naming another author's events: %v", err)
	}
	waitForIngest(t, db, del.ID, true)
	waitForIngest(t, db, theirs.ID, true)
}

// NIP-01 treats a missing or empty d tag as the empty string, so an
// addressable event without one is stored, and no-d, ["d"] and ["d",""]
// versions replace one another.
func TestAddressable_EmptyD(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()
	priv, pub := newTestKey(t)
	now := time.Now().Unix()

	noD := signEvent(t, priv, pub, 30789, "v1", nil, now-30)
	if err := db.StoreEvent(ctx, noD); err != nil {
		t.Fatalf("no d tag: %v", err)
	}
	waitForIngest(t, db, noD.ID, true)

	bare := signEvent(t, priv, pub, 30789, "v2", [][]string{{"d"}}, now-20)
	if err := db.StoreEvent(ctx, bare); err != nil {
		t.Fatalf(`["d"]: %v`, err)
	}
	waitForIngest(t, db, bare.ID, true)
	waitForIngest(t, db, noD.ID, false)

	empty := signEvent(t, priv, pub, 30789, "v3", [][]string{{"d", ""}}, now-10)
	if err := db.StoreEvent(ctx, empty); err != nil {
		t.Fatalf(`["d",""]: %v`, err)
	}
	waitForIngest(t, db, empty.ID, true)
	waitForIngest(t, db, bare.ID, false)

	older := signEvent(t, priv, pub, 30789, "older", nil, now-40)
	if err := db.StoreEvent(ctx, older); err == nil || !strings.HasPrefix(err.Error(), "blocked: ") {
		t.Fatalf("older version at the empty address: err = %v, want blocked:", err)
	}

	// A distinct d value is a different address.
	named := signEvent(t, priv, pub, 30789, "named", [][]string{{"d", "x"}}, now-5)
	if err := db.StoreEvent(ctx, named); err != nil {
		t.Fatal(err)
	}
	waitForIngest(t, db, named.ID, true)

	// "kind:pubkey:" names the empty address.
	del := signEvent(t, priv, pub, 5, "", [][]string{{"a", "30789:" + pub + ":"}}, now)
	if err := db.ProcessDeletion(ctx, del); err != nil {
		t.Fatal(err)
	}
	waitForIngest(t, db, empty.ID, false)
	waitForIngest(t, db, named.ID, true)
}
