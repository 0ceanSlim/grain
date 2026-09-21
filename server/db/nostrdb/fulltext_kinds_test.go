package nostrdb

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// search runs a NIP-50 query constrained to one kind and returns the ids.
func search(t *testing.T, db *NDB, query string, kind int) []string {
	t.Helper()
	txn, err := db.BeginQuery()
	if err != nil {
		t.Fatalf("begin query: %v", err)
	}
	defer txn.EndQuery()
	got, err := txn.TextSearch(query, nostr.Filter{Kinds: []int{kind}}, 10)
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	ids := make([]string, 0, len(got))
	for _, e := range got {
		ids = append(ids, e.ID)
	}
	return ids
}

// Issue #71: the fulltext kind set is configurable and defaults to
// {0, 1, 30023}. A kind-0 profile is searchable out of the box; a kind
// outside the set is not until an operator opts it in.
func TestFulltextKinds_DefaultAndOptIn(t *testing.T) {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	pub := hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey()))
	ctx := context.Background()
	ts := time.Now().Unix()

	profile := signEvent(t, priv, pub, 0, `{"name":"quokka","about":"marsupial enjoyer"}`, nil, ts)
	reaction := signEvent(t, priv, pub, 7, "platypus approves", [][]string{{"e", "0000000000000000000000000000000000000000000000000000000000000001"}}, ts)

	t.Run("default_indexes_profiles_not_reactions", func(t *testing.T) {
		db := openTempDB(t) // Open -> DefaultFulltextKinds
		for _, e := range []nostr.Event{profile, reaction} {
			if err := db.StoreEvent(ctx, e); err != nil {
				t.Fatalf("store kind %d: %v", e.Kind, err)
			}
			waitForIngest(t, db, e.ID, true)
		}
		if got := search(t, db, "quokka", 0); len(got) != 1 || got[0] != profile.ID {
			t.Fatalf("kind-0 search: got %v, want [%s]", got, profile.ID)
		}
		if got := search(t, db, "platypus", 7); len(got) != 0 {
			t.Fatalf("kind-7 search on default set: got %v, want none", got)
		}
	})

	t.Run("opt_in_kind", func(t *testing.T) {
		db, err := OpenWithOptions(t.TempDir(), Options{
			MapSizeMB:     32,
			IngestThreads: 1,
			FulltextKinds: []int{7},
		})
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		t.Cleanup(db.Close)
		for _, e := range []nostr.Event{profile, reaction} {
			if err := db.StoreEvent(ctx, e); err != nil {
				t.Fatalf("store kind %d: %v", e.Kind, err)
			}
			waitForIngest(t, db, e.ID, true)
		}
		if got := search(t, db, "platypus", 7); len(got) != 1 || got[0] != reaction.ID {
			t.Fatalf("kind-7 search after opt-in: got %v, want [%s]", got, reaction.ID)
		}
		if got := search(t, db, "quokka", 0); len(got) != 0 {
			t.Fatalf("kind-0 search with set {7}: got %v, want none", got)
		}
	})

	t.Run("too_many_kinds_rejected", func(t *testing.T) {
		kinds := make([]int, MaxFulltextKinds+1)
		for i := range kinds {
			kinds[i] = i
		}
		if _, err := OpenWithOptions(t.TempDir(), Options{MapSizeMB: 32, FulltextKinds: kinds}); err == nil {
			t.Fatal("expected an error for 65 fulltext kinds")
		}
	})
}
