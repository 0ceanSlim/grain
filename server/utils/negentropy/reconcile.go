package negentropy

import (
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

	// Initiate reconciliation
	query, err := neg.Initiate()
	if err != nil {
		if err.Error() == "already initiated" {
			log.Printf("Reconciliation already initiated. Skipping initiation.")
		} else {
			log.Fatalf("Failed to initiate reconciliation: %v", err)
		}
	}
	log.Printf("Generated query for reconciliation: %x", query)

	// Perform reconciliation
	var haveIds, needIds []string
	_, err = neg.ReconcileWithIDs(query, &haveIds, &needIds)
	if err != nil {
		log.Fatalf("Reconciliation failed: %v", err)
	}
	log.Printf("Reconciliation completed. Have IDs: %v, Need IDs: %v", haveIds, needIds)

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
