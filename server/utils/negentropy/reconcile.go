package negentropy

import (
	"encoding/hex"
	"fmt"
	"log"

	nostr "grain/server/types"

	"github.com/illuzen/go-negentropy"
)

// reconcileStorage performs reconciliation using populated custom storage.
func reconcileStorage(haveStorage, needStorage *CustomStorage) []nostr.Event {
	// Create a Negentropy instance
	neg, err := negentropy.NewNegentropy(haveStorage, 4096) // Frame size: 4096
	if err != nil {
		log.Fatalf("Failed to create Negentropy instance: %v", err)
	}

	// Set initiator mode explicitly
	neg.SetInitiator()

	// Initiate reconciliation
	query, err := neg.Initiate()
	if err != nil {
		if err.Error() == "already initiated" {
			log.Printf("Reconciliation already initiated. Skipping initiation.")
		} else {
			log.Fatalf("Failed to initiate reconciliation: %v", err)
		}
	} else {
		log.Printf("Generated query for reconciliation: length=%d, hex=%s", len(query), hex.EncodeToString(query))
	}

	// Log an overview of storage
	log.Printf("haveStorage: %d items, needStorage: %d items", len(haveStorage.items), len(needStorage.items))

	// Perform reconciliation
	var haveIds, needIds []string
	_, err = neg.ReconcileWithIDs(query, &haveIds, &needIds)
	if err != nil {
		log.Printf("Reconciliation failed. Query: %s", hex.EncodeToString(query))
		log.Fatalf("Reconciliation failed: %v", err)
	}

	log.Printf("Reconciliation completed. Have IDs: %d, Need IDs: %d", len(haveIds), len(needIds))

	// Generate the reconciled dataset
	var reconciledEvents []nostr.Event
	for _, evt := range needStorage.items {
		if contains(needIds, string(evt.ID)) {
			reconciledEvents = append(reconciledEvents, nostr.Event{
				ID:        string(evt.ID),
				CreatedAt: int64(evt.Timestamp),
			})
		}
	}

	return reconciledEvents
}

// Helper to check if a slice contains a value
func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// decodeHexID decodes a 64-character hexadecimal string into a 32-byte binary slice.
func decodeHexID(hexID string) ([]byte, error) {
	if len(hexID) != 64 {
		return nil, fmt.Errorf("invalid hex ID length: expected 64, got %d (ID: %s)", len(hexID), hexID)
	}
	return hex.DecodeString(hexID)
}
