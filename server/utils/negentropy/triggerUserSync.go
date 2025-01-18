package negentropy

import (
	"log"
	"sort"

	configTypes "grain/config/types"
	nostr "grain/server/types"
)

// triggerUserSync fetches Kind 10002 events and stores the latest one.
func triggerUserSync(pubKey string, negentropyCfg *configTypes.NegentropyConfig, serverCfg *configTypes.ServerConfig) {
	log.Printf("Starting user sync for pubkey: %s", pubKey)

	initialRelays := negentropyCfg.InitialSyncRelays
	if len(initialRelays) == 0 {
		log.Println("No initial relays configured for user sync.")
		return
	}

	events := fetchKind10002Events(pubKey, initialRelays)

	if len(events) == 0 {
		log.Printf("No Kind 10002 events found for pubkey: %s", pubKey)
		return
	}

	// Sort events by `created_at` descending
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt > events[j].CreatedAt
	})

	// Select the newest event
	latestEvent := events[0]
	log.Printf("Selected latest Kind 10002 event: ID=%s, CreatedAt=%d", latestEvent.ID, latestEvent.CreatedAt)

	// Forward the event to the local relay using ServerConfig for port
	err := storeUserRelays(latestEvent, serverCfg)
	if err != nil {
		log.Printf("Failed to forward Kind 10002 event to local relay: %v", err)
		return
	}

	log.Printf("Kind 10002 event successfully stored for pubkey: %s", pubKey)

	// Trigger the next step to aggregate user outbox events
	aggregateUserOutbox(pubKey, latestEvent)
}

// aggregateUserOutbox starts the process of aggregating user outbox events.
func aggregateUserOutbox(pubKey string, relayEvent nostr.Event) {
	// Extract relay URLs from the tags of the Kind 10002 event
	var relayURLs []string
	for _, tag := range relayEvent.Tags {
		if len(tag) > 1 && tag[0] == "r" {
			relayURLs = append(relayURLs, tag[1])
		}
	}

	log.Printf("Triggering aggregation of user outbox events for pubkey: %s from relays: %v", pubKey, relayURLs)
	// Placeholder: Implement logic for aggregating user outbox events
}
