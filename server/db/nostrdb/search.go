package nostrdb

/*
#include "nostrdb.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// maxTextSearchResults mirrors MAX_TEXT_SEARCH_RESULTS in nostrdb.h.
// nostrdb won't return more than this per call regardless of the
// configured limit; the REQ-side paging loop is what stitches multiple
// calls together to honor a higher effective limit.
const maxTextSearchResults = 128

// TextSearch runs a NIP-50 fulltext query within an existing
// transaction. `base` carries the non-search constraints (kinds,
// authors, tags, since, until); its Search field is ignored — the
// query string is passed as a separate argument to ndb_text_search_with.
//
// Result ordering is descending by created_at (newest-first), matching
// the rest of grain's read paths. nostrdb only indexes content for
// kinds 1 and 30023 — searches that filter to other kinds will return
// nothing even if matching content exists in the DB.
func (txn *Txn) TextSearch(query string, base nostr.Filter, limit int) ([]nostr.Event, error) {
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = maxTextSearchResults
	}
	if limit > maxTextSearchResults {
		limit = maxTextSearchResults
	}

	// Strip any search field on the base filter before lowering it to
	// nostrdb — search is the separate function arg, not a filter
	// component. filterToJSON already excludes it (see query.go), but
	// be explicit so a future refactor can't accidentally double-feed
	// the search term.
	base.Search = ""

	var ndbFilter C.struct_ndb_filter
	if err := buildSingleNDBFilter(&ndbFilter, base); err != nil {
		return nil, fmt.Errorf("failed to build ndb filter for search: %w", err)
	}
	defer C.ndb_filter_destroy(&ndbFilter)

	var cfg C.struct_ndb_text_search_config
	C.ndb_default_text_search_config(&cfg)
	C.ndb_text_search_config_set_order(&cfg, C.NDB_ORDER_DESCENDING)
	C.ndb_text_search_config_set_limit(&cfg, C.int(limit))

	var results C.struct_ndb_text_search_results

	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	rc := C.ndb_text_search_with(&txn.txn, cQuery, &results, &cfg, &ndbFilter)
	if rc == 0 {
		// nostrdb returns 0 for "no results" as well as transient
		// errors; treat as empty rather than propagating since we
		// can't distinguish, and an error here would close the REQ.
		return nil, nil
	}

	count := int(results.num_results)
	log.GetLogger("db-search").Debug("Text search executed",
		"query", query, "results", count, "limit", limit)

	events := make([]nostr.Event, 0, count)
	for i := 0; i < count; i++ {
		r := &results.results[i]
		if r.note == nil {
			continue
		}
		evt := noteToEventDirect(r.note)
		events = append(events, evt)
	}
	return events, nil
}

// TextSearch is the no-transaction convenience wrapper, mirroring Query.
func (db *NDB) TextSearch(query string, base nostr.Filter, limit int) ([]nostr.Event, error) {
	txn, err := db.BeginQuery()
	if err != nil {
		return nil, err
	}
	defer txn.EndQuery()
	return txn.TextSearch(query, base, limit)
}
