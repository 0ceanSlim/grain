package nostrdb

import (
	"context"
	"fmt"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"

	"encoding/hex"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// TestFilterScratchBuffer_LargeAuthorPrefixArray exercises the production
// case from the v0.5.4 logs: a REQ with 256 two-character author prefixes
// ("00".."ff"). With the original 16KB scratch buffer in
// buildSingleNDBFilter, ndb_filter_from_json overran the buffer and
// returned 0, so the query was rejected with "ndb_filter_from_json
// failed". The fix sizes the buffer dynamically (max(64KB, 4× JSON)). If
// this regresses, the test fails fast with the canonical error.
//
// The original fix landed in 9784b70 and was inadvertently reverted in
// 4a6650f along with an unrelated noteToEvent change. Restored alongside
// this test.
func TestFilterScratchBuffer_LargeAuthorPrefixArray(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()

	// Seed the DB with at least one event so the query path actually
	// touches the filter parser. Pubkey value doesn't matter — the
	// filter contains 256 two-char prefixes, so practically any event
	// matches.
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	pub := hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey()))
	evt := signEvent(t, priv, pub, 1, "filter buffer test", nil, time.Now().Unix())
	if err := db.StoreEvent(ctx, evt); err != nil {
		t.Fatalf("store: %v", err)
	}
	waitForIngest(t, db, evt.ID, true)

	// Build the prefix list: 256 two-char strings "00".."ff".
	prefixes := make([]string, 256)
	for i := 0; i < 256; i++ {
		prefixes[i] = fmt.Sprintf("%02x", i)
	}

	limit := 50
	filters := []nostr.Filter{{
		Authors: prefixes,
		Limit:   &limit,
	}}

	// The behaviour we're guarding against: the filter parser
	// returning 0 (rendered to the caller as "ndb_filter_from_json
	// failed for: ...") because the scratch buffer was too small.
	if _, err := db.Query(filters, limit); err != nil {
		t.Fatalf("Query with 256 author prefixes failed: %v", err)
	}
}

// TestFilterScratchBuffer_LargeFullPubkeyArray covers the other
// production shape: a REQ with hundreds of full 64-char pubkeys. v0.5.4
// production logs showed REQs with 384+ full pubkeys hitting the same
// "ndb_filter_from_json failed" path.
func TestFilterScratchBuffer_LargeFullPubkeyArray(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()

	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	pub := hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey()))
	evt := signEvent(t, priv, pub, 1, "filter buffer test", nil, time.Now().Unix())
	if err := db.StoreEvent(ctx, evt); err != nil {
		t.Fatalf("store: %v", err)
	}
	waitForIngest(t, db, evt.ID, true)

	// 400 distinct synthetic 64-char hex pubkeys. We don't need them
	// to match anything — we just need the filter JSON to be large
	// enough to overrun the prior 16KB scratch buffer.
	pubkeys := make([]string, 400)
	for i := 0; i < 400; i++ {
		pubkeys[i] = fmt.Sprintf("%064x", i)
	}

	limit := 50
	filters := []nostr.Filter{{
		Authors: pubkeys,
		Kinds:   []int{1, 30023, 6, 9802, 7, 30315},
		Limit:   &limit,
	}}

	if _, err := db.Query(filters, limit); err != nil {
		t.Fatalf("Query with 400 full pubkeys + 6 kinds failed: %v", err)
	}
}

// TestFilterScratchBuffer_NormalFilterStillFits is a smoke test confirming
// the fix didn't break tiny filters (they should not even allocate beyond
// the 64KB floor). Quick to run, catches accidental regressions in the
// "small filter" fast path.
func TestFilterScratchBuffer_NormalFilterStillFits(t *testing.T) {
	db := openTempDB(t)
	ctx := context.Background()

	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	pub := hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey()))
	evt := signEvent(t, priv, pub, 1, "small filter test", nil, time.Now().Unix())
	if err := db.StoreEvent(ctx, evt); err != nil {
		t.Fatalf("store: %v", err)
	}
	waitForIngest(t, db, evt.ID, true)

	limit := 10
	filters := []nostr.Filter{{
		Authors: []string{pub},
		Kinds:   []int{1},
		Limit:   &limit,
	}}

	results, err := db.Query(filters, limit)
	if err != nil {
		t.Fatalf("Query with single-pubkey filter failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected at least one result, got 0")
	}
}
