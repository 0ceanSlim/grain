package negentropy

import (
	"log"

	nostr "grain/server/types"

	"github.com/illuzen/go-negentropy"
)

// reconcile processes "haves" and "needs" datasets using Negentropy for reconciliation.
func reconcile(haves, needs []nostr.Event) []nostr.Event {
	// Prepare a fresh custom storage for Negentropy
	haveStorage := &CustomStorage{}
	needStorage := &CustomStorage{}

	// Populate "haves" into the storage
	for _, evt := range haves {
		item := negentropy.NewItem(uint64(evt.CreatedAt), []byte(evt.ID))
		haveStorage.items = append(haveStorage.items, *item)
	}

	// Populate "needs" into the storage
	for _, evt := range needs {
		item := negentropy.NewItem(uint64(evt.CreatedAt), []byte(evt.ID))
		needStorage.items = append(needStorage.items, *item)
	}

	// Create a Negentropy instance
	neg, err := negentropy.NewNegentropy(haveStorage, 4096) // Frame size: 4096
	if err != nil {
		log.Fatalf("Failed to create Negentropy instance: %v", err)
	}

	// Initiate reconciliation
	query, err := neg.Initiate()
	if err != nil {
		log.Fatalf("Failed to initiate reconciliation: %v", err)
	}
	log.Printf("Generated query for reconciliation: %+v", query)

	// Perform reconciliation
	var haveIds, needIds []string
	_, err = neg.ReconcileWithIDs(query, &haveIds, &needIds)
	if err != nil {
		log.Fatalf("Reconciliation failed: %v", err)
	}
	log.Printf("Reconciliation completed. Have IDs: %v, Need IDs: %v", haveIds, needIds)

	// Generate the reconciled dataset
	var reconciledEvents []nostr.Event
	for _, evt := range needs {
		if contains(needIds, evt.ID) {
			reconciledEvents = append(reconciledEvents, evt)
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
