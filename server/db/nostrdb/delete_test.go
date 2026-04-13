package nostrdb

import (
	"context"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"

	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	raw, _ := json.Marshal([]interface{}{0, evt.PubKey, evt.CreatedAt, evt.Kind, evt.Tags, evt.Content})
	h := sha256.Sum256(raw)
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
