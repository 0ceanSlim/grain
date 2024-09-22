package mongo

import (
	"context"
	types "grain/config/types"
	"grain/server/utils"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func PurgeOldEvents(cfg *types.EventPurgeConfig, whitelist []string) {
	if !cfg.Enabled {
		return
	}

	client := GetClient()
	collection := client.Database("grain").Collection("events")

	// Calculate the cutoff time
	cutoff := time.Now().AddDate(0, 0, -cfg.KeepDurationDays).Unix()

	filter := bson.M{
		"created_at": bson.M{"$lt": cutoff}, // Filter older events
	}

	if cfg.ExcludeWhitelisted && len(whitelist) > 0 {
		filter["pubkey"] = bson.M{"$nin": whitelist} // Exclude whitelisted pubkeys
	}

	// Handle purging by category
	for category, purge := range cfg.PurgeByCategory {
		if purge {
			filter["category"] = category
			_, err := collection.DeleteMany(context.TODO(), filter)
			if err != nil {
				log.Printf("Error purging events by category %s: %v", category, err)
			}
		}
	}

	// Handle purging by kind
	for _, kindRule := range cfg.PurgeByKind {
		if kindRule.Enabled {
			filter["kind"] = kindRule.Kind
			_, err := collection.DeleteMany(context.TODO(), filter)
			if err != nil {
				log.Printf("Error purging events by kind %d: %v", kindRule.Kind, err)
			}
		}
	}
}

// Example of a periodic purging task
// ScheduleEventPurging runs the event purging at a configurable interval.
func ScheduleEventPurging(cfg *types.ServerConfig) {
	// Use the purge interval from the configuration
	purgeInterval := time.Duration(cfg.EventPurge.PurgeIntervalHours) * time.Hour
	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()

	for range ticker.C {
		whitelist := getWhitelistedPubKeys(cfg)
		PurgeOldEvents(&cfg.EventPurge, whitelist)
	}
}

// Fetch whitelisted pubkeys from both the config and any additional domains.
func getWhitelistedPubKeys(cfg *types.ServerConfig) []string {
	whitelistedPubkeys := cfg.PubkeyWhitelist.Pubkeys

	// Fetch pubkeys from domains if domain whitelist is enabled
	if cfg.DomainWhitelist.Enabled {
		domains := cfg.DomainWhitelist.Domains
		pubkeys, err := utils.FetchPubkeysFromDomains(domains)
		if err != nil {
			log.Printf("Error fetching pubkeys from domains: %v", err)
			return whitelistedPubkeys // Return existing whitelisted pubkeys in case of error
		}
		// Append fetched pubkeys from domains to the whitelisted pubkeys
		whitelistedPubkeys = append(whitelistedPubkeys, pubkeys...)
	}

	return whitelistedPubkeys
}
