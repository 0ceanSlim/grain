package negentropy

import (
	"fmt"
	"log"
	"sort"

	configTypes "grain/config/types"

	"github.com/illuzen/go-negentropy"
)

// triggerUserSync fetches Kind 10002 events and stores the latest one.
func triggerUserSync(pubKey string, negentropyCfg *configTypes.NegentropyConfig, serverCfg *configTypes.ServerConfig) {
	log.Printf("Starting user sync for pubkey: %s", pubKey)

	initialRelays := negentropyCfg.InitialSyncRelays
	if len(initialRelays) == 0 {
		log.Println("No initial relays configured for user sync.")
		return
	}

	// Fetch user outboxes from the initial relays
	userOutboxEvents := fetchUserOutboxes(pubKey, initialRelays)
	if len(userOutboxEvents) == 0 {
		log.Printf("No Kind 10002 events found for pubkey: %s", pubKey)
		return
	}

	// Sort user outbox events by `created_at` descending
	sort.Slice(userOutboxEvents, func(i, j int) bool {
		return userOutboxEvents[i].CreatedAt > userOutboxEvents[j].CreatedAt
	})

	// Select the newest outbox event
	latestOutboxEvent := userOutboxEvents[0]
	log.Printf("Selected latest Kind 10002 event: ID=%s, CreatedAt=%d", latestOutboxEvent.ID, latestOutboxEvent.CreatedAt)

	// Forward the event to the local relay
	err := storeUserOutboxes(latestOutboxEvent, serverCfg)
	if err != nil {
		log.Printf("Failed to forward Kind 10002 event to local relay: %v", err)
		return
	}

	log.Printf("Kind 10002 event successfully stored for pubkey: %s", pubKey)

	// Extract user outbox relays from the tags of the latest event
	var userOutboxes []string
	for _, tag := range latestOutboxEvent.Tags {
		if len(tag) > 1 && tag[0] == "r" {
			userOutboxes = append(userOutboxes, tag[1])
		}
	}

	if len(userOutboxes) == 0 {
		log.Printf("No outbox relays found in the latest Kind 10002 event for pubkey: %s", pubKey)
		return
	}

	// Fetch "haves" from the local relay
	haves, err := fetchHaves(pubKey, fmt.Sprintf("ws://localhost%s", serverCfg.Server.Port))
	if err != nil {
		log.Printf("Failed to fetch haves from local relay: %v", err)
		return
	}
	log.Printf("Fetched %d events from the local relay (haves).", len(haves))

	// Fetch "needs" from the user outbox relays
	needs := fetchNeeds(pubKey, userOutboxes)
	log.Printf("Fetched %d events from the user outbox relays (needs).", len(needs))

	// Populate custom storages
	haveStorage := &CustomStorage{}
	needStorage := &CustomStorage{}

	for _, evt := range haves {
		decodedID, err := decodeHexID(evt.ID)
		if err != nil {
			log.Fatalf("Failed to decode ID in haves: %v", err)
		}
		item := negentropy.NewItem(uint64(evt.CreatedAt), decodedID)
		haveStorage.items = append(haveStorage.items, *item)
	}

	for _, evt := range needs {
		decodedID, err := decodeHexID(evt.ID)
		if err != nil {
			log.Fatalf("Failed to decode ID in needs: %v", err)
		}
		item := negentropy.NewItem(uint64(evt.CreatedAt), decodedID)
		needStorage.items = append(needStorage.items, *item)
	}

	log.Printf("Populated custom storage: haveStorage (%d items), needStorage (%d items).",
		len(haveStorage.items), len(needStorage.items))

	if err := haveStorage.ValidateIDs(); err != nil {
		log.Fatalf("Invalid ID in haveStorage: %v", err)
	}
	if err := needStorage.ValidateIDs(); err != nil {
		log.Fatalf("Invalid ID in needStorage: %v", err)
	}

	// Perform reconciliation
	reconciledEvents := reconcileStorage(haveStorage, needStorage)

	// Log the reconciled dataset for debugging
	log.Printf("Reconciled dataset for pubkey %s: %+v", pubKey, reconciledEvents)
}
