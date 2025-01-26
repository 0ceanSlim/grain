package negentropy

import (
	"log"

	nostr "grain/server/types"

	"github.com/illuzen/go-negentropy"
)

// aggregateUserOutboxEvents fetches all events and performs negentropy-based reconciliation.
func aggregateUserOutboxEvents(pubKey string, relayEvent nostr.Event) {
	// Extract relay URLs from the Kind 10002 event
	var relayURLs []string
	for _, tag := range relayEvent.Tags {
		if len(tag) > 1 && tag[0] == "r" {
			relayURLs = append(relayURLs, tag[1])
		}
	}

	if len(relayURLs) == 0 {
		log.Printf("No outbox relays found for pubkey: %s", pubKey)
		return
	}

	log.Printf("Fetching events for pubkey: %s from outbox relays: %v", pubKey, relayURLs)

	// Fetch events from the outbox relays
	events := fetchAllUserEvents(pubKey, relayURLs)
	if len(events) == 0 {
		log.Printf("No events found for pubkey: %s from any outbox relay", pubKey)
		return
	}

	log.Printf("Fetched %d events for pubkey: %s. Starting reconciliation.", len(events), pubKey)

	for attempt := 1; ; attempt++ {
		// Prepare a fresh custom storage for reconciliation
		storage := &CustomStorage{}
		for _, evt := range events {
			item := negentropy.NewItem(uint64(evt.CreatedAt), []byte(evt.ID))
			storage.items = append(storage.items, *item)
		}

		// Create a fresh Negentropy instance
		neg, err := negentropy.NewNegentropy(storage, 4096) // Frame size: 4096
		if err != nil {
			log.Fatalf("Failed to create Negentropy instance: %v", err)
		}

		// Explicitly set the initiator flag
		if attempt == 1 {
			neg.SetInitiator()
		}

		// Initiate reconciliation
		query, err := neg.Initiate()
		if err != nil {
			if err.Error() == "already initiated" && attempt < 3 {
				log.Printf("Reconciliation initiation failed: %v. Retrying attempt %d.", err, attempt)
				continue
			}
			log.Fatalf("Failed to initiate reconciliation: %v", err)
		}
		log.Printf("Generated query for reconciliation: %+v", query)

		// Perform reconciliation
		var haveIds, needIds []string
		response, err := neg.ReconcileWithIDs(query, &haveIds, &needIds)
		if err != nil {
			log.Printf("Reconciliation failed on attempt %d: %v", attempt, err)
			if attempt < 3 {
				log.Println("Retrying reconciliation with a new instance.")
				continue
			}
			log.Fatalf("Reconciliation failed after %d attempts: %v", attempt, err)
		}

		// Log success
		log.Printf("Reconciliation completed successfully. Have IDs: %v, Need IDs: %v", haveIds, needIds)
		log.Printf("Final response from reconciliation: %v", response)
		break
	}

	// Process synced events (if necessary)
	// You can fetch the "need IDs" and update the database or perform additional operations here.
}
