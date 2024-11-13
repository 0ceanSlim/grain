package mongo

import (
	"context"
	"grain/config"
	types "grain/config/types"
	nostr "grain/server/types"
	"grain/server/utils"
	"log"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// PurgeOldEvents removes old events based on the configuration and a list of whitelisted pubkeys.
func PurgeOldEvents(cfg *types.EventPurgeConfig) {
	if !cfg.Enabled {
		return
	}

	client := GetClient()
	cutoff := time.Now().Add(-time.Duration(cfg.KeepIntervalHours) * time.Hour).Unix()
	var collectionsToPurge []string

	// Determine collections to purge
	if cfg.PurgeByKindEnabled {
		for _, kind := range cfg.KindsToPurge {
			collectionsToPurge = append(collectionsToPurge, "event-kind"+strconv.Itoa(kind))
		}
	} else {
		// If `purge_by_kind_enabled` is false, add all potential event kinds or find dynamically
		collectionsToPurge = getAllEventCollections(client)
	}

	for _, collectionName := range collectionsToPurge {
		collection := client.Database("grain").Collection(collectionName)
		baseFilter := bson.M{"created_at": bson.M{"$lt": cutoff}}

		cursor, err := collection.Find(context.TODO(), baseFilter)
		if err != nil {
			log.Printf("Error fetching old events for purging from %s: %v", collectionName, err)
			continue
		}
		defer cursor.Close(context.TODO())

		for cursor.Next(context.TODO()) {
			var evt nostr.Event
			if err := cursor.Decode(&evt); err != nil {
				log.Printf("Error decoding event from %s: %v", collectionName, err)
				continue
			}

			// Skip if the pubkey is whitelisted
			if cfg.ExcludeWhitelisted && config.IsPubKeyWhitelisted(evt.PubKey) {
				log.Printf("Skipping purging for whitelisted event ID: %s, pubkey: %s", evt.ID, evt.PubKey)
				continue
			}

			// Check if purging by category is enabled and if the event matches the allowed category
			category := utils.DetermineEventCategory(evt.Kind)
			if purge, exists := cfg.PurgeByCategory[category]; exists && purge {
				_, err := collection.DeleteOne(context.TODO(), bson.M{"id": evt.ID})
				if err != nil {
					log.Printf("Error purging event ID %s from %s: %v", evt.ID, collectionName, err)
				} else {
					log.Printf("Purged event ID: %s from %s", evt.ID, collectionName)
				}
			}
		}
	}
}

// getAllEventCollections returns a list of all event collections if purging all kinds.
func getAllEventCollections(client *mongo.Client) []string {
	var collections []string
	collectionNames, err := client.Database("grain").ListCollectionNames(context.TODO(), bson.M{})
	if err != nil {
		log.Printf("Error listing collection names: %v", err)
		return collections
	}

	for _, name := range collectionNames {
		if len(name) > 10 && name[:10] == "event-kind" {
			collections = append(collections, name)
		}
	}
	return collections
}

// ScheduleEventPurging runs the event purging at a configurable interval.
func ScheduleEventPurging(cfg *types.ServerConfig) {
	purgeInterval := time.Duration(cfg.EventPurge.PurgeIntervalMinutes) * time.Minute
	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()

	for range ticker.C {
		PurgeOldEvents(&cfg.EventPurge)
		log.Println("Scheduled purging completed.")
	}
}
