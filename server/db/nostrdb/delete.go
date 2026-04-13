package nostrdb

/*
#include "nostrdb.h"
#include <stdlib.h>
*/
import "C"
import (
	"context"
	"fmt"
	"strconv"
	"strings"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ProcessDeletion handles NIP-09 kind 5 deletion events. It walks the event's
// `e` and `a` tags, enforces NIP-09's same-pubkey authorization rule at each
// tag, physically removes matching events via the nostrdb writer thread
// (ndb_request_delete_note), then stores the kind-5 record itself so clients
// can still see the deletion marker per spec.
//
// Tag processing is best-effort: a failure on one tag is logged and the next
// tag is still processed. The kind-5 is always ingested at the end so that a
// client querying for it still sees it, even if none of its targets existed.
func (db *NDB) ProcessDeletion(ctx context.Context, evt nostr.Event) error {
	log.GetLogger("db-store").Info("Processing deletion event",
		"event_id", evt.ID,
		"pubkey", evt.PubKey,
		"tag_count", len(evt.Tags))

	for _, tag := range evt.Tags {
		if len(tag) < 2 {
			continue
		}

		switch tag[0] {
		case "e":
			// Author requested delete of a specific event by ID.
			if err := db.verifyAndDeleteByID(tag[1], evt.PubKey, evt.CreatedAt); err != nil {
				log.GetLogger("db-store").Error("Failed to delete event by ID",
					"target_event_id", tag[1],
					"event_id", evt.ID,
					"error", err)
			}

		case "a":
			// Author requested delete by addressable coordinate
			// "kind:pubkey:d-tag". NIP-09 requires the coordinate's pubkey
			// to match the deleter, same as the `e` tag ownership rule.
			if err := db.verifyAndDeleteByAddr(tag[1], evt.PubKey, evt.CreatedAt); err != nil {
				log.GetLogger("db-store").Error("Failed to delete addressable event",
					"coord", tag[1],
					"event_id", evt.ID,
					"error", err)
			}
		}
	}

	// Store the deletion event itself — per NIP-09 the kind-5 record stays
	// visible so clients can see the deletion marker.
	if err := db.ingestEvent(evt); err != nil {
		return fmt.Errorf("failed to store deletion event: %w", err)
	}

	log.GetLogger("db-store").Info("Deletion event processed", "event_id", evt.ID)
	return nil
}

// verifyAndDeleteByID enforces NIP-09's same-pubkey rule and physically removes
// the target event. If the target doesn't exist this is a silent no-op. If the
// target exists but was created *after* the deletion event, the delete is
// refused (NIP-09's past-only semantics — a later event at the same ID would
// have a different ID, but we apply the same rule defensively).
func (db *NDB) verifyAndDeleteByID(eventID, requesterPubKey string, deleteCreatedAt int64) error {
	txn, err := db.BeginQuery()
	if err != nil {
		return err
	}

	target, err := txn.GetNoteByID(eventID)
	txn.EndQuery()
	if err != nil {
		return fmt.Errorf("error looking up event %s: %w", eventID, err)
	}

	if target == nil {
		log.GetLogger("db-store").Debug("Deletion target not found", "event_id", eventID)
		return nil
	}

	// NIP-09: deletion only applies to events by the same author.
	if target.PubKey != requesterPubKey {
		log.GetLogger("db-store").Warn("Deletion rejected - pubkey mismatch",
			"event_id", eventID,
			"event_pubkey", target.PubKey,
			"requester_pubkey", requesterPubKey)
		return fmt.Errorf("cannot delete event owned by another pubkey")
	}

	// NIP-09 is past-only: a deletion request cannot remove an event that
	// was created after the deletion event's own created_at.
	if target.CreatedAt > deleteCreatedAt {
		log.GetLogger("db-store").Debug("Deletion skipped - target is newer than delete event",
			"event_id", eventID,
			"target_created_at", target.CreatedAt,
			"delete_created_at", deleteCreatedAt)
		return nil
	}

	idBytes, err := hexToBytes32(eventID)
	if err != nil {
		return fmt.Errorf("invalid target id %s: %w", eventID, err)
	}
	var id32 [32]byte
	copy(id32[:], idBytes)

	if err := db.DeleteNoteByID(id32); err != nil {
		return fmt.Errorf("ndb delete failed: %w", err)
	}

	log.GetLogger("db-store").Info("Event deleted",
		"event_id", eventID, "pubkey", requesterPubKey)
	return nil
}

// verifyAndDeleteByAddr parses a NIP-09 `a` tag value of the form
// "kind:pubkey:d-tag", enforces that the coordinate's pubkey matches the
// deleter (NIP-09 same-author rule — this check was entirely missing before),
// queries nostrdb for events at that coordinate with created_at <= the
// deletion event's timestamp, and physically deletes each match.
func (db *NDB) verifyAndDeleteByAddr(coord, requesterPubKey string, deleteCreatedAt int64) error {
	parts := strings.SplitN(coord, ":", 3)
	if len(parts) != 3 {
		return fmt.Errorf("malformed a tag value %q", coord)
	}
	kindStr, coordPubkey, dTag := parts[0], parts[1], parts[2]

	kind, err := strconv.Atoi(kindStr)
	if err != nil {
		return fmt.Errorf("invalid kind in a tag %q: %w", coord, err)
	}

	// NIP-09 same-author rule for addressable deletes.
	if coordPubkey != requesterPubKey {
		log.GetLogger("db-store").Warn("Addressable deletion rejected - pubkey mismatch",
			"coord", coord,
			"coord_pubkey", coordPubkey,
			"requester_pubkey", requesterPubKey)
		return fmt.Errorf("cannot delete addressable event owned by another pubkey")
	}

	// Query the coordinate. For non-addressable kinds (replaceable 0/3/1xxxx)
	// the `d` tag is absent — drop it from the filter in that case.
	limit := 100
	filter := nostr.Filter{
		Authors: []string{coordPubkey},
		Kinds:   []int{kind},
		Limit:   &limit,
	}
	if isAddressable(kind) {
		filter.Tags = map[string][]string{"d": {dTag}}
	}

	matches, err := db.Query([]nostr.Filter{filter}, limit)
	if err != nil {
		return fmt.Errorf("coordinate query failed: %w", err)
	}

	deleted := 0
	for _, m := range matches {
		// Past-only: a later event at the same coordinate survives.
		if m.CreatedAt > deleteCreatedAt {
			continue
		}
		idBytes, err := hexToBytes32(m.ID)
		if err != nil {
			log.GetLogger("db-store").Warn("Skipping bad id in coord match",
				"coord", coord, "id", m.ID, "error", err)
			continue
		}
		var id32 [32]byte
		copy(id32[:], idBytes)
		if err := db.DeleteNoteByID(id32); err != nil {
			log.GetLogger("db-store").Error("Addressable delete failed",
				"coord", coord, "event_id", m.ID, "error", err)
			continue
		}
		deleted++
	}

	log.GetLogger("db-store").Info("Addressable events deleted",
		"coord", coord,
		"deleted", deleted,
		"scanned", len(matches))
	return nil
}
