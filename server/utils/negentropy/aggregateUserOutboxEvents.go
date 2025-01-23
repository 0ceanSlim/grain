package negentropy

import (
	nostr "grain/server/types"
	"log"
)

// aggregateUserOutbox fetches all events authored by the user from their outbox relays.
func aggregateUserOutboxEvents(pubKey string, relayEvent nostr.Event) {
	// Extract relay URLs from the Kind 10002 event
	var relayURLs []string
	for _, tag := range relayEvent.Tags {
		if len(tag) > 1 && tag[0] == "r" {
			// Include only relays marked as "write" or unspecified (not "read")
			if len(tag) == 2 || (len(tag) > 2 && tag[2] == "write") {
				relayURLs = append(relayURLs, tag[1])
			}
		}
	}

	if len(relayURLs) == 0 {
		log.Printf("No outbox relays found for pubkey: %s", pubKey)
		return
	}

	log.Printf("Fetching events for pubkey: %s from outbox relays: %v", pubKey, relayURLs)

	// Fetch events from each outbox relay
	events := fetchAllUserEvents(pubKey, relayURLs)

	if len(events) == 0 {
		log.Printf("No events found for pubkey: %s from any outbox relay", pubKey)
		return
	}

	log.Printf("Fetched %d events for pubkey: %s. Ready for further processing.", len(events), pubKey)

	// Placeholder for the next step: compare the sets using Negentropy sync logic
	// processNegentropyComparison(events)
}
