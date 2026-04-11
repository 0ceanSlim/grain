package nostrdb

/*
#include "nostrdb.h"
#include <stdlib.h>
*/
import "C"
import (
	"context"
	"fmt"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// StoreEvent processes and stores a Nostr event in the database.
// nostrdb handles event ingestion internally including parsing and indexing.
// For replaceable and addressable events, we must handle the replacement
// semantics ourselves since nostrdb does not enforce NIP-01 replacement rules.
func (db *NDB) StoreEvent(ctx context.Context, evt nostr.Event) error {
	category := determineEventCategory(evt.Kind)

	log.GetLogger("db-store").Debug("Processing event for storage",
		"event_id", evt.ID,
		"kind", evt.Kind,
		"category", category,
		"pubkey", evt.PubKey)

	switch {
	case evt.Kind == 2:
		// Deprecated event kind
		log.GetLogger("db-store").Debug("Ignoring deprecated event kind 2", "event_id", evt.ID)
		return nil

	case evt.Kind >= 20000 && evt.Kind < 30000:
		// Ephemeral events are not stored
		log.GetLogger("db-store").Info("Ephemeral event received and ignored",
			"event_id", evt.ID, "kind", evt.Kind)
		return nil

	case isReplaceable(evt.Kind):
		return db.storeReplaceable(ctx, evt)

	case isAddressable(evt.Kind):
		return db.storeAddressable(ctx, evt)

	default:
		// Regular events and unknown kinds: just ingest
		return db.ingestEvent(evt)
	}
}

// ingestEvent feeds a raw event JSON into nostrdb for storage.
func (db *NDB) ingestEvent(evt nostr.Event) error {
	jsonStr, err := eventToJSON(evt)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Wrap in ["EVENT", <event>] format that ndb_process_event expects
	wrapped := `["EVENT","` + evt.ID[:8] + `",` + jsonStr + `]`
	if err := db.ProcessEvent(wrapped); err != nil {
		return fmt.Errorf("failed to ingest event kind %d: %w", evt.Kind, err)
	}

	log.GetLogger("db-store").Info("Event stored",
		"event_id", evt.ID, "kind", evt.Kind, "pubkey", evt.PubKey)
	return nil
}

// storeReplaceable handles NIP-01 replaceable events (kinds 0, 3, 10000-19999).
// Only the most recent event per (pubkey, kind) is kept.
func (db *NDB) storeReplaceable(ctx context.Context, evt nostr.Event) error {
	txn, err := db.BeginQuery()
	if err != nil {
		return fmt.Errorf("failed to begin query for replaceable check: %w", err)
	}

	// Look for existing event with same pubkey and kind
	limit := 1
	filters := []nostr.Filter{{
		Authors: []string{evt.PubKey},
		Kinds:   []int{evt.Kind},
		Limit:   &limit,
	}}

	existing, err := txn.Query(filters, 1)
	txn.EndQuery()

	if err != nil {
		return fmt.Errorf("failed to query existing replaceable event: %w", err)
	}

	if len(existing) > 0 {
		old := existing[0]
		// Reject if existing is newer, or same timestamp with lower ID (NIP-01 tiebreak)
		if old.CreatedAt > evt.CreatedAt || (old.CreatedAt == evt.CreatedAt && old.ID < evt.ID) {
			log.GetLogger("db-store").Info("Rejecting replaceable event - newer version exists",
				"event_id", evt.ID, "existing_id", old.ID, "kind", evt.Kind)
			return fmt.Errorf("blocked: relay already has a newer event of the same kind with this pubkey")
		}
		// TODO: nostrdb doesn't have a direct delete API.
		// The old event will be superseded by the new one in query results
		// since queries return by created_at desc. For true replacement,
		// we'd need nostrdb to support note deletion or flagging.
	}

	return db.ingestEvent(evt)
}

// storeAddressable handles NIP-01 parameterized replaceable events (kinds 30000-39999).
// Only the most recent event per (pubkey, kind, d-tag) is kept.
func (db *NDB) storeAddressable(ctx context.Context, evt nostr.Event) error {
	// Extract d-tag
	var dTag string
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "d" {
			dTag = tag[1]
			break
		}
	}

	if dTag == "" {
		return fmt.Errorf("no d tag present in addressable event")
	}

	txn, err := db.BeginQuery()
	if err != nil {
		return fmt.Errorf("failed to begin query for addressable check: %w", err)
	}

	// Look for existing event with same pubkey, kind, and d-tag
	limit := 1
	filters := []nostr.Filter{{
		Authors: []string{evt.PubKey},
		Kinds:   []int{evt.Kind},
		Tags:    map[string][]string{"d": {dTag}},
		Limit:   &limit,
	}}

	existing, err := txn.Query(filters, 1)
	txn.EndQuery()

	if err != nil {
		return fmt.Errorf("failed to query existing addressable event: %w", err)
	}

	if len(existing) > 0 {
		old := existing[0]
		if old.CreatedAt > evt.CreatedAt || (old.CreatedAt == evt.CreatedAt && old.ID < evt.ID) {
			log.GetLogger("db-store").Info("Rejecting addressable event - newer version exists",
				"event_id", evt.ID, "existing_id", old.ID,
				"kind", evt.Kind, "d_tag", dTag)
			return fmt.Errorf("blocked: relay already has a newer event for this pubkey and dTag")
		}
	}

	return db.ingestEvent(evt)
}

// isReplaceable returns true for NIP-01 replaceable event kinds.
func isReplaceable(kind int) bool {
	return kind == 0 || kind == 3 || (kind >= 10000 && kind < 20000)
}

// isAddressable returns true for NIP-01 parameterized replaceable event kinds.
func isAddressable(kind int) bool {
	return kind >= 30000 && kind < 40000
}

// determineEventCategory returns a human-readable category for a kind.
func determineEventCategory(kind int) string {
	switch {
	case kind == 0 || kind == 3 || (kind >= 10000 && kind < 20000):
		return "replaceable"
	case kind >= 20000 && kind < 30000:
		return "ephemeral"
	case kind >= 30000 && kind < 40000:
		return "addressable"
	case kind == 5:
		return "deletion"
	default:
		return "regular"
	}
}
