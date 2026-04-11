package nostrdb

/*
#include "nostrdb.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"
import (
	"encoding/json"
	"fmt"
	"unsafe"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// maxQueryResults is the maximum number of results a single query can return.
const maxQueryResults = 10000

// Query executes NIP-01 filters against the database and returns matching events.
// This opens and closes a read transaction internally.
func (db *NDB) Query(filters []nostr.Filter, limit int) ([]nostr.Event, error) {
	txn, err := db.BeginQuery()
	if err != nil {
		return nil, err
	}
	defer txn.EndQuery()

	return txn.Query(filters, limit)
}

// Query executes NIP-01 filters within an existing transaction.
func (txn *Txn) Query(filters []nostr.Filter, limit int) ([]nostr.Event, error) {
	if len(filters) == 0 {
		return nil, nil
	}

	if limit <= 0 {
		limit = 1000
	}
	if limit > maxQueryResults {
		limit = maxQueryResults
	}

	// Build nostrdb filters from our Filter type
	ndbFilters, err := buildNDBFilters(filters)
	if err != nil {
		return nil, fmt.Errorf("failed to build ndb filters: %w", err)
	}
	defer func() {
		for i := range ndbFilters {
			C.ndb_filter_destroy(&ndbFilters[i])
		}
	}()

	// Execute query
	results := make([]C.struct_ndb_query_result, limit)
	var count C.int

	rc := C.ndb_query(
		&txn.txn,
		&ndbFilters[0],
		C.int(len(ndbFilters)),
		&results[0],
		C.int(limit),
		&count,
	)

	if rc == 0 {
		return nil, fmt.Errorf("ndb_query failed")
	}

	log.GetLogger("db-query").Debug("Query executed",
		"filter_count", len(filters),
		"results", int(count),
		"limit", limit)

	// Convert results to Go events
	events := make([]nostr.Event, 0, int(count))
	for i := 0; i < int(count); i++ {
		result := results[i]
		if result.note == nil {
			continue
		}

		evt := noteToEventDirect(result.note)
		events = append(events, evt)
	}

	return events, nil
}

// GetNoteByID looks up a single event by its hex ID.
func (txn *Txn) GetNoteByID(hexID string) (*nostr.Event, error) {
	idBytes, err := hexToBytes32(hexID)
	if err != nil {
		return nil, fmt.Errorf("invalid event ID: %w", err)
	}

	var noteSize C.uint64_t
	var noteKey C.uint64_t

	note := C.ndb_get_note_by_id(
		&txn.txn,
		(*C.uchar)(unsafe.Pointer(&idBytes[0])),
		(*C.size_t)(unsafe.Pointer(&noteSize)),
		&noteKey,
	)

	if note == nil {
		return nil, nil // not found
	}

	evt := noteToEventDirect(note)
	return &evt, nil
}

// CheckDuplicateEvent checks if an event with the given ID already exists.
func (db *NDB) CheckDuplicateEvent(evt nostr.Event) (bool, error) {
	txn, err := db.BeginQuery()
	if err != nil {
		// If we can't query, allow the event through
		log.GetLogger("db").Warn("Failed to begin query for duplicate check, allowing event",
			"event_id", evt.ID, "error", err)
		return false, nil
	}
	defer txn.EndQuery()

	existing, err := txn.GetNoteByID(evt.ID)
	if err != nil {
		log.GetLogger("db").Warn("Error during duplicate check, allowing event",
			"event_id", evt.ID, "error", err)
		return false, nil
	}

	if existing != nil {
		log.GetLogger("db").Info("Duplicate event found",
			"event_id", evt.ID, "kind", evt.Kind, "pubkey", evt.PubKey)
		return true, nil
	}

	return false, nil
}

// GetAllAuthors returns all unique pubkeys that have events in the database.
func (db *NDB) GetAllAuthors() []string {
	// Query a broad set of events and collect unique pubkeys
	limit := 50000
	filters := []nostr.Filter{{Limit: &limit}}

	events, err := db.Query(filters, limit)
	if err != nil {
		log.GetLogger("db").Error("Failed to query events for author list", "error", err)
		return nil
	}

	pubkeySet := make(map[string]struct{})
	for _, evt := range events {
		pubkeySet[evt.PubKey] = struct{}{}
	}

	authors := make([]string, 0, len(pubkeySet))
	for pk := range pubkeySet {
		authors = append(authors, pk)
	}

	log.GetLogger("db").Info("Fetched unique authors", "count", len(authors))
	return authors
}

// buildNDBFilters converts Go nostr.Filter slice to C ndb_filter array.
func buildNDBFilters(filters []nostr.Filter) ([]C.struct_ndb_filter, error) {
	ndbFilters := make([]C.struct_ndb_filter, len(filters))

	for i, filter := range filters {
		if err := buildSingleNDBFilter(&ndbFilters[i], filter); err != nil {
			// Clean up already-built filters on error
			for j := 0; j < i; j++ {
				C.ndb_filter_destroy(&ndbFilters[j])
			}
			return nil, fmt.Errorf("filter %d: %w", i, err)
		}
	}

	return ndbFilters, nil
}

// buildSingleNDBFilter converts a single Go Filter to a C ndb_filter.
// Uses ndb_filter_from_json for simplicity and correctness.
func buildSingleNDBFilter(nf *C.struct_ndb_filter, filter nostr.Filter) error {
	// Initialize the filter's internal buffers before parsing
	if C.ndb_filter_init(nf) == 0 {
		return fmt.Errorf("ndb_filter_init failed")
	}

	// Serialize the filter to JSON, then let nostrdb parse it.
	// This is the most robust approach since nostrdb handles all the
	// filter field semantics internally.
	filterJSON, err := filterToJSON(filter)
	if err != nil {
		C.ndb_filter_destroy(nf)
		return fmt.Errorf("failed to serialize filter: %w", err)
	}

	cJSON := C.CString(filterJSON)
	defer C.free(unsafe.Pointer(cJSON))

	// ndb_filter_from_json needs a scratch buffer for IDs/strings
	bufSize := 1024 * 16 // 16KB scratch
	buf := (*C.uchar)(C.malloc(C.size_t(bufSize)))
	defer C.free(unsafe.Pointer(buf))

	rc := C.ndb_filter_from_json(cJSON, C.int(len(filterJSON)), nf, buf, C.int(bufSize))
	if rc == 0 {
		C.ndb_filter_destroy(nf)
		return fmt.Errorf("ndb_filter_from_json failed for: %s", filterJSON)
	}

	return nil
}

// filterToJSON serializes a nostr.Filter to JSON in the standard NIP-01 format.
func filterToJSON(f nostr.Filter) (string, error) {
	m := make(map[string]interface{})

	if len(f.IDs) > 0 {
		m["ids"] = f.IDs
	}
	if len(f.Authors) > 0 {
		m["authors"] = f.Authors
	}
	if len(f.Kinds) > 0 {
		m["kinds"] = f.Kinds
	}
	if f.Since != nil {
		m["since"] = f.Since.Unix()
	}
	if f.Until != nil {
		m["until"] = f.Until.Unix()
	}
	if f.Limit != nil {
		m["limit"] = *f.Limit
	}

	// Tag filters
	for key, values := range f.Tags {
		if len(values) > 0 {
			tagKey := key
			if len(tagKey) > 0 && tagKey[0] != '#' {
				tagKey = "#" + tagKey
			}
			m[tagKey] = values
		}
	}

	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
