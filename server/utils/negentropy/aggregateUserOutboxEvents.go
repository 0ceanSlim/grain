package negentropy

import (
	"fmt"
	nostr "grain/server/types"
	"log"

	"github.com/illuzen/go-negentropy"
)

// CustomStorage implements the negentropy.Storage interface.
type CustomStorage struct {
	items []negentropy.Item
}

func (s *CustomStorage) Size() int {
	return len(s.items)
}

func (s *CustomStorage) GetItem(i uint64) (negentropy.Item, error) {
	if int(i) >= len(s.items) {
		return negentropy.Item{}, fmt.Errorf("index out of bounds")
	}
	return s.items[i], nil
}

func (s *CustomStorage) Iterate(begin, end int, cb func(item negentropy.Item, i int) bool) error {
	for i := begin; i < end; i++ {
		if !cb(s.items[i], i) {
			break
		}
	}
	return nil
}

func (s *CustomStorage) FindLowerBound(begin, end int, value negentropy.Bound) (int, error) {
	for i := begin; i < end; i++ {
		if !s.items[i].LessThan(value.Item) {
			return i, nil
		}
	}
	return end, nil
}

func (s *CustomStorage) Fingerprint(begin, end int) (negentropy.Fingerprint, error) {
	if begin < 0 || end > len(s.items) || begin > end {
		return negentropy.Fingerprint{}, fmt.Errorf("invalid range for fingerprint: begin=%d, end=%d", begin, end)
	}

	// Initialize an empty fingerprint (assuming it's a struct).
	fingerprint := negentropy.Fingerprint{}

	// Build a fingerprint based on XOR of IDs (custom logic, adjust as needed).
	for i := begin; i < end; i++ {
		for j := range s.items[i].ID {
			if len(fingerprint) <= j {
				fingerprint = append(fingerprint, 0)
			}
			fingerprint[j] ^= s.items[i].ID[j]
		}
	}

	return fingerprint, nil
}

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

	// Prepare custom storage for reconciliation
	storage := &CustomStorage{}
	for _, evt := range events {
		item := negentropy.NewItem(uint64(evt.CreatedAt), []byte(evt.ID))
		storage.items = append(storage.items, *item)
	}

	// Create a Negentropy instance
	neg, err := negentropy.NewNegentropy(storage, 4096) // Frame size: 4096
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
	response, err := neg.Reconcile(query)
	if err != nil {
		log.Fatalf("Reconciliation failed: %v", err)
	}

	log.Printf("Reconciliation completed successfully. Synced events: %v", response)
}
