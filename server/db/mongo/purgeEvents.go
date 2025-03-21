package mongo

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/0ceanslim/grain/config"
	types "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// PurgeOldEvents removes old events based on the configuration and a list of whitelisted pubkeys.
func PurgeOldEvents(cfg *types.EventPurgeConfig) {
	if !cfg.Enabled {
		return
	}

	client := GetClient()
	dbName := GetDatabaseName() // ✅ Use dynamic database name

	currentTime := time.Now().Unix()
	cutoff := currentTime - int64(cfg.KeepIntervalHours*3600) // Convert hours to seconds

	var collectionsToPurge []string
	totalPurged := 0
	totalKept := 0

	// Determine collections to purge
	if cfg.PurgeByKindEnabled {
		for _, kind := range cfg.KindsToPurge {
			collectionsToPurge = append(collectionsToPurge, fmt.Sprintf("event-kind%d", kind))
		}
	} else {
		collectionsToPurge = getAllEventCollections(client)
	}

	for _, collectionName := range collectionsToPurge {
		collection := client.Database(dbName).Collection(collectionName) // ✅ Use dynamic DB name
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
				totalKept++
				continue
			}

			if evt.CreatedAt >= cutoff {
				totalKept++
				continue
			}

			if cfg.ExcludeWhitelisted && config.IsPubKeyWhitelisted(evt.PubKey, true) {
				totalKept++
				continue
			}

			category := utils.DetermineEventCategory(evt.Kind)
			if purge, exists := cfg.PurgeByCategory[category]; !exists || !purge {
				totalKept++
				continue
			}

			_, err = collection.DeleteOne(context.TODO(), bson.M{"id": evt.ID})
			if err != nil {
				log.Printf("Error purging event ID %s from %s: %v", evt.ID, collectionName, err)
				totalKept++
			} else {
				totalPurged++
			}
		}
	}

	log.Printf("Purging completed: Total events purged = %d, Total events kept = %d", totalPurged, totalKept)
}

// getAllEventCollections returns a list of all event collections if purging all kinds.
func getAllEventCollections(client *mongo.Client) []string {
	var collections []string
	dbName := GetDatabaseName() // ✅ Use dynamic database name

	collectionNames, err := client.Database(dbName).ListCollectionNames(context.TODO(), bson.M{})
	if err != nil {
		log.Printf("Error listing collection names: %v", err)
		return collections
	}

	for _, name := range collectionNames {
		if strings.HasPrefix(name, "event-kind") {
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
		//log.Println("Scheduled purging completed.")
	}
}
