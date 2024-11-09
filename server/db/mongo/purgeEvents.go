package mongo

import (
	"context"
	"grain/config"
	types "grain/config/types"
	nostr "grain/server/types"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

// PurgeOldEvents removes old events based on the configuration and a list of whitelisted pubkeys.
func PurgeOldEvents(cfg *types.EventPurgeConfig) {
	if !cfg.Enabled {
		return
	}

	client := GetClient()
	collection := client.Database("grain").Collection("events")

	// Calculate the cutoff time
	cutoff := time.Now().AddDate(0, 0, -cfg.KeepDurationDays).Unix()

	// Create the base filter for fetching old events
	baseFilter := bson.M{
		"created_at": bson.M{"$lt": cutoff}, // Filter for events older than the cutoff
	}

	cursor, err := collection.Find(context.TODO(), baseFilter)
	if err != nil {
		log.Printf("Error fetching old events for purging: %v", err)
		return
	}
	defer cursor.Close(context.TODO())

	for cursor.Next(context.TODO()) {
		var evt nostr.Event
		if err := cursor.Decode(&evt); err != nil {
			log.Printf("Error decoding event: %v", err)
			continue
		}

		// Check if the event's pubkey is whitelisted and skip purging if configured to do so
		if cfg.ExcludeWhitelisted && config.IsPubKeyWhitelisted(evt.PubKey) {
			log.Printf("Skipping purging for whitelisted event ID: %s, pubkey: %s", evt.ID, evt.PubKey)
			continue
		}

		// Proceed with deleting the event if it is not whitelisted
		_, err := collection.DeleteOne(context.TODO(), bson.M{"id": evt.ID})
		if err != nil {
			log.Printf("Error purging event ID %s: %v", evt.ID, err)
		} else {
			log.Printf("Purged event ID: %s", evt.ID)
		}
	}
}

// ScheduleEventPurging runs the event purging at a configurable interval.
func ScheduleEventPurging(cfg *types.ServerConfig) {
	purgeInterval := time.Duration(cfg.EventPurge.PurgeIntervalHours) * time.Hour
	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()

	for range ticker.C {
		PurgeOldEvents(&cfg.EventPurge)
		log.Println("Scheduled purging completed.")
	}
}
