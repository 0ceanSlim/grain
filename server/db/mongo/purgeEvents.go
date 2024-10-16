package mongo

import (
	"context"
	"grain/config"
	types "grain/config/types"
	"grain/server/utils"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// PurgeOldEvents removes old events based on the configuration and a list of whitelisted pubkeys.
func PurgeOldEvents(cfg *types.EventPurgeConfig, whitelist []string) {
	if !cfg.Enabled {
		return
	}

	client := GetClient()
	collection := client.Database("grain").Collection("events")

	// Calculate the cutoff time
	cutoff := time.Now().AddDate(0, 0, -cfg.KeepDurationDays).Unix()

	// Create the filter for purging old events
	filter := bson.M{
		"created_at": bson.M{"$lt": cutoff}, // Filter older events
	}

	// Exclude whitelisted pubkeys if specified in the config
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

// ScheduleEventPurging runs the event purging at a configurable interval.
func ScheduleEventPurging(cfg *types.ServerConfig) {
	// Use the purge interval from the configuration
	purgeInterval := time.Duration(cfg.EventPurge.PurgeIntervalHours) * time.Hour
	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()

	for range ticker.C {
		// Fetch the whitelisted pubkeys without passing cfg directly
		whitelist := getWhitelistedPubKeys()
		PurgeOldEvents(&cfg.EventPurge, whitelist)
		log.Printf("Purged old events, keeping whitelisted pubkeys: %v", whitelist)
	}
}

// Fetch whitelisted pubkeys from both the whitelist config and any additional domains.
func getWhitelistedPubKeys() []string {
	// Get the whitelist configuration
	whitelistCfg := config.GetWhitelistConfig()
	if whitelistCfg == nil {
		log.Println("whitelistCfg is nil, returning an empty list of whitelisted pubkeys.")
		return []string{}
	}

	// Start with the statically defined pubkeys
	whitelistedPubkeys := whitelistCfg.PubkeyWhitelist.Pubkeys

	// Fetch pubkeys from domains if domain whitelist is enabled
	if whitelistCfg.DomainWhitelist.Enabled {
		domains := whitelistCfg.DomainWhitelist.Domains
		pubkeys, err := utils.FetchPubkeysFromDomains(domains)
		if err != nil {
			log.Printf("Error fetching pubkeys from domains: %v", err)
			// Return the existing statically whitelisted pubkeys in case of an error
			return whitelistedPubkeys
		}
		// Append fetched pubkeys from domains to the whitelisted pubkeys
		whitelistedPubkeys = append(whitelistedPubkeys, pubkeys...)
	}

	return whitelistedPubkeys
}