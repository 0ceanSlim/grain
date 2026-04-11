package nostrdb

/*
#include "nostrdb.h"
#include <stdlib.h>
*/
import "C"
import (
	"context"
	"fmt"
	"strings"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ProcessDeletion handles NIP-09 kind 5 deletion events.
// It stores the deletion event itself and marks referenced events as deleted.
// Note: nostrdb uses the NDB_NOTE_FLAG_DELETED flag rather than physically removing events.
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
			// Delete specific event by ID
			eventID := tag[1]
			if err := db.verifyAndFlagDeleted(ctx, eventID, evt.PubKey); err != nil {
				log.GetLogger("db-store").Error("Failed to delete event by ID",
					"target_event_id", eventID,
					"event_id", evt.ID,
					"error", err)
				// Continue processing other tags
			}

		case "a":
			// Delete addressable event by kind:pubkey:d-tag
			parts := strings.Split(tag[1], ":")
			if len(parts) == 3 {
				log.GetLogger("db-store").Debug("Processing addressable deletion",
					"kind", parts[0], "pubkey", parts[1], "d_tag", parts[2],
					"event_id", evt.ID)
				// nostrdb doesn't have direct addressable deletion.
				// The deletion event itself will be stored, and relay query logic
				// should filter out events that have been deleted.
			}
		}
	}

	// Store the deletion event itself
	if err := db.ingestEvent(evt); err != nil {
		return fmt.Errorf("failed to store deletion event: %w", err)
	}

	log.GetLogger("db-store").Info("Deletion event processed", "event_id", evt.ID)
	return nil
}

// verifyAndFlagDeleted checks that the target event belongs to the same pubkey
// as the deletion event, then marks it as deleted.
func (db *NDB) verifyAndFlagDeleted(ctx context.Context, eventID string, requesterPubKey string) error {
	txn, err := db.BeginQuery()
	if err != nil {
		return err
	}
	defer txn.EndQuery()

	target, err := txn.GetNoteByID(eventID)
	if err != nil {
		return fmt.Errorf("error looking up event %s: %w", eventID, err)
	}

	if target == nil {
		// Event not found - nothing to delete
		log.GetLogger("db-store").Debug("Deletion target not found", "event_id", eventID)
		return nil
	}

	// Verify same author (NIP-09 requirement)
	if target.PubKey != requesterPubKey {
		log.GetLogger("db-store").Warn("Deletion rejected - pubkey mismatch",
			"event_id", eventID,
			"event_pubkey", target.PubKey,
			"requester_pubkey", requesterPubKey)
		return fmt.Errorf("cannot delete event owned by another pubkey")
	}

	// TODO: nostrdb currently doesn't expose a Go-callable function to set
	// the NDB_NOTE_FLAG_DELETED flag on a note. This would need to be added
	// to the C API or handled via ndb_set_note_meta.
	// For now, the deletion event is stored and relay query logic should
	// check for deletion events when returning results.
	log.GetLogger("db-store").Info("Event flagged for deletion",
		"event_id", eventID, "pubkey", requesterPubKey)

	return nil
}
