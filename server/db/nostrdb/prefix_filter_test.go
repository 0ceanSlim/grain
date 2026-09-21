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

// Issue #72: NIP-01 lets `ids` and `authors` be hex prefixes. The fork's
// parser used to reject anything shorter than 64 chars, which failed the
// whole REQ. This is the Go-visible half: a prefix REQ returns exactly the
// events under that prefix, and the id ordering of a prefix scan is
// normalized to newest-first by Query.
func TestPrefixFilters_IDsAndAuthors(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()

	// two authors; keys are random, so find a prefix that separates them
	privA, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	privB, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	pubA := hex.EncodeToString(schnorr.SerializePubKey(privA.PubKey()))
	pubB := hex.EncodeToString(schnorr.SerializePubKey(privB.PubKey()))
	n := 1
	for n < 64 && pubA[:n] == pubB[:n] {
		n++
	}
	prefixA := pubA[:n]

	base := time.Now().Unix() - 10
	evts := []nostr.Event{
		signEvent(t, privA, pubA, 1, "a one", nil, base+1),
		signEvent(t, privA, pubA, 1, "a two", nil, base+2),
		signEvent(t, privB, pubB, 1, "b one", nil, base+3),
	}
	for _, e := range evts {
		if err := db.StoreEvent(ctx, e); err != nil {
			t.Fatalf("store: %v", err)
		}
		waitForIngest(t, db, e.ID, true)
	}

	ids := func(f nostr.Filter) []string {
		t.Helper()
		got, err := db.Query([]nostr.Filter{f}, 10)
		if err != nil {
			t.Fatalf("query %+v: %v", f, err)
		}
		out := make([]string, len(got))
		for i, e := range got {
			out[i] = e.ID
		}
		return out
	}

	// author prefix: both of A's notes, newest first, none of B's
	got := ids(nostr.Filter{Authors: []string{prefixA}})
	if len(got) != 2 || got[0] != evts[1].ID || got[1] != evts[0].ID {
		t.Fatalf("authors[%q]: got %v, want [%s %s]", prefixA, got, evts[1].ID, evts[0].ID)
	}

	// odd-length prefix of an id
	idPrefix := evts[2].ID[:5]
	got = ids(nostr.Filter{IDs: []string{idPrefix}})
	if len(got) != 1 || got[0] != evts[2].ID {
		t.Fatalf("ids[%q]: got %v, want [%s]", idPrefix, got, evts[2].ID)
	}

	// mixed list: a prefix for A plus B's full pubkey, with kinds
	got = ids(nostr.Filter{Authors: []string{prefixA, pubB}, Kinds: []int{1}})
	if len(got) != 3 {
		t.Fatalf("mixed authors: got %d events, want 3", len(got))
	}

	// a one-char prefix neither key starts with
	other := ""
	for _, c := range "0123456789abcdef" {
		if byte(c) != pubA[0] && byte(c) != pubB[0] {
			other = string(c)
			break
		}
	}
	if got = ids(nostr.Filter{Authors: []string{other}}); len(got) != 0 {
		t.Fatalf("authors[%q]: got %v, want none", other, got)
	}
}
