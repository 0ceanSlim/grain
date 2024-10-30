package blossom

import (
	"context"
	"fmt"
	"log"
	"os"

	"grain/config"
	serverconfig "grain/config/types"
	"grain/server/db/mongo"
	"grain/server/db/mongo/kinds"
	types "grain/server/types"
	"grain/server/utils"

	"github.com/fiatjaf/khatru"
	"github.com/nbd-wtf/go-nostr"
	"github.com/spf13/afero"
)

var (
	cfg = config.GetConfig() // Load configuration globally once
	fs  = afero.NewOsFs()    // File system abstraction using afero
)

// InitBlossom initializes the Blossom server with the configured settings.
func InitBlossom(cfg *serverconfig.ServerConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	if cfg.Blossom.BlossomPath == "" {
		return fmt.Errorf("BlossomPath is not set in config")
	}

	err := os.MkdirAll(cfg.Blossom.BlossomPath, 0755)
	if err != nil {
		return fmt.Errorf("error creating blossom directory: %w", err)
	}

	relay := khatru.NewRelay()

	// Initialize MongoDB connection
	dbClient, err := mongo.InitDB(cfg)
	if err != nil {
		return fmt.Errorf("error initializing MongoDB: %w", err)
	}

	// Store events in MongoDB using StoreMongoEvent
	relay.StoreEvent = append(relay.StoreEvent, func(ctx context.Context, event *nostr.Event) error {
		relayEvent := convertToRelayEvent(*event)
		mongo.StoreMongoEvent(ctx, relayEvent, nil)
		return nil
	})

	// Query events from MongoDB
	relay.QueryEvents = append(relay.QueryEvents, func(ctx context.Context, filter nostr.Filter) (chan *nostr.Event, error) {
		relayFilters := convertToRelayFilters([]nostr.Filter{filter})
		results, err := mongo.QueryEvents(relayFilters, dbClient, cfg.MongoDB.Database)
		if err != nil {
			return nil, err
		}

		ch := make(chan *nostr.Event)
		go func() {
			defer close(ch)
			for _, evt := range results {
				nostrEvent := convertToNostrEvent(evt)
				ch <- &nostrEvent
			}
		}()
		return ch, nil
	})

	// Handle deletions via kind 5 events using HandleDeleteKind
	relay.DeleteEvent = append(relay.DeleteEvent, func(ctx context.Context, event *nostr.Event) error {
		if event.Kind == 5 {
			relayEvent := convertToRelayEvent(*event)
			return kinds.HandleDeleteKind(ctx, relayEvent, dbClient, nil)
		}
		return nil
	})

	// Reject uploads if pubkey is not whitelisted
	relay.RejectEvent = append(relay.RejectEvent, func(ctx context.Context, event *nostr.Event) (bool, string) {
		if !isPubKeyWhitelisted(event.PubKey) {
			return true, "pubkey is not whitelisted"
		}
		return false, ""
	})

	return nil
}

// Check if the event's pubkey is whitelisted
func isPubKeyWhitelisted(pubkey string) bool {
	whitelistCfg := config.GetWhitelistConfig()
	if whitelistCfg == nil {
		log.Println("Whitelist configuration not loaded.")
		return false
	}

	// Check static whitelist first
	for _, whitelistedPubKey := range whitelistCfg.PubkeyWhitelist.Pubkeys {
		if pubkey == whitelistedPubKey {
			return true
		}
	}

	// Dynamically fetch pubkeys from whitelisted domains, if configured
	if whitelistCfg.DomainWhitelist.Enabled {
		domainPubkeys, err := utils.FetchPubkeysFromDomains(whitelistCfg.DomainWhitelist.Domains)
		if err != nil {
			log.Println("Error fetching pubkeys from domains:", err)
			return false
		}
		for _, domainPubkey := range domainPubkeys {
			if pubkey == domainPubkey {
				return true
			}
		}
	}

	return false
}

// Convert nostr.Event to types.Event
func convertToRelayEvent(nEvent nostr.Event) types.Event {
	tags := make([][]string, len(nEvent.Tags))
	for i, tag := range nEvent.Tags {
		tags[i] = []string(tag)
	}

	return types.Event{
		ID:        nEvent.ID,
		PubKey:    nEvent.PubKey,
		CreatedAt: nEvent.CreatedAt.Time().Unix(),
		Kind:      nEvent.Kind,
		Tags:      tags, // Use the converted tags
		Content:   nEvent.Content,
		Sig:       nEvent.Sig,
	}
}

// Convert types.Event to nostr.Event
func convertToNostrEvent(rEvent types.Event) nostr.Event {
	tags := make(nostr.Tags, len(rEvent.Tags))
	for i, tag := range rEvent.Tags {
		tags[i] = nostr.Tag(tag)
	}

	return nostr.Event{
		ID:        rEvent.ID,
		PubKey:    rEvent.PubKey,
		CreatedAt: nostr.Timestamp(rEvent.CreatedAt),
		Kind:      rEvent.Kind,
		Tags:      tags, // Use the converted tags
		Content:   rEvent.Content,
		Sig:       rEvent.Sig,
	}
}

// Convert []nostr.Filter to []types.Filter
func convertToRelayFilters(nFilters []nostr.Filter) []types.Filter {
	var relayFilters []types.Filter
	for _, nFilter := range nFilters {
		relayFilters = append(relayFilters, types.Filter{
			IDs:     utils.ToStringArray(nFilter.IDs), // Use the utility function for array conversion
			Authors: utils.ToStringArray(nFilter.Authors),
			Kinds:   utils.ToIntArray(nFilter.Kinds), // Convert kinds to int array using utility
			Tags:    utils.ToTagsMap(nFilter.Tags),   // Use utility to convert tags
			Since:   utils.ToTime(nFilter.Since),     // Convert *nostr.Timestamp to *time.Time
			Until:   utils.ToTime(nFilter.Until),     // Convert *nostr.Timestamp to *time.Time
			Limit:   utils.ToInt(nFilter.Limit),      // Convert int to *int
		})
	}
	return relayFilters
}
