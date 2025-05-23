package mongo

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/0ceanslim/grain/config"
	cfgTypes "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Set the logging component for MongoDB purge operations
func purgeLog() *slog.Logger {
	return utils.GetLogger("mongo-purge")
}


// PurgeOldEvents removes old events based on the configuration and a list of whitelisted pubkeys.
func PurgeOldEvents(cfg *cfgTypes.EventPurgeConfig) {
	if !cfg.Enabled {
		purgeLog().Debug("Event purging is disabled")
		return
	}

	purgeLog().Info("Starting event purge", 
		"keep_hours", cfg.KeepIntervalHours,
		"exclude_whitelisted", cfg.ExcludeWhitelisted,
		"purge_by_kind_enabled", cfg.PurgeByKindEnabled)

	client := GetClient()
	dbName := GetDatabaseName()

	currentTime := time.Now().Unix()
	cutoff := currentTime - int64(cfg.KeepIntervalHours*3600) // Convert hours to seconds
	cutoffTime := time.Unix(cutoff, 0)

	purgeLog().Debug("Purge cutoff calculated", 
		"current_time", time.Unix(currentTime, 0).Format(time.RFC3339),
		"cutoff_time", cutoffTime.Format(time.RFC3339),
		"cutoff_unix", cutoff)

	var collectionsToPurge []string
	totalPurged := 0
	totalKept := 0
	collectionStats := make(map[string]map[string]int) // Maps collection name to stats

	// Determine collections to purge
	if cfg.PurgeByKindEnabled {
		purgeLog().Debug("Using kind-specific purging", "kinds_to_purge", cfg.KindsToPurge)
		for _, kind := range cfg.KindsToPurge {
			collectionName := fmt.Sprintf("event-kind%d", kind)
			collectionsToPurge = append(collectionsToPurge, collectionName)
			// Initialize stats tracking for this collection
			collectionStats[collectionName] = map[string]int{
				"purged": 0,
				"kept":   0,
				"errors": 0,
			}
		}
	} else {
		purgeLog().Debug("Using category-based purging")
		collectionsToPurge = getAllEventCollections(client)
		// Initialize stats tracking for all collections
		for _, name := range collectionsToPurge {
			collectionStats[name] = map[string]int{
				"purged": 0,
				"kept":   0,
				"errors": 0,
			}
		}
	}

	purgeLog().Info("Identified collections for purging", 
		"collection_count", len(collectionsToPurge),
		"category_purging", cfg.PurgeByCategory)

	for _, collectionName := range collectionsToPurge {
		stats := collectionStats[collectionName]
		collection := client.Database(dbName).Collection(collectionName)
		baseFilter := bson.M{"created_at": bson.M{"$lt": cutoff}}

		// Get count before purging (for logging)
		count, err := collection.CountDocuments(context.TODO(), baseFilter)
		if err != nil {
			purgeLog().Error("Error counting documents for purging", 
				"collection", collectionName, 
				"error", err)
			stats["errors"]++
			continue
		}

		purgeLog().Debug("Found candidates for purging", 
			"collection", collectionName, 
			"count", count)

		cursor, err := collection.Find(context.TODO(), baseFilter)
		if err != nil {
			purgeLog().Error("Error fetching old events for purging", 
				"collection", collectionName, 
				"error", err)
			stats["errors"]++
			continue
		}
		defer cursor.Close(context.TODO())

		for cursor.Next(context.TODO()) {
			var evt nostr.Event
			if err := cursor.Decode(&evt); err != nil {
				purgeLog().Error("Error decoding event", 
					"collection", collectionName, 
					"error", err)
				stats["kept"]++
				totalKept++
				continue
			}

			// Double-check created_at (should be redundant with our query, but safety first)
			if evt.CreatedAt >= cutoff {
				purgeLog().Debug("Event too recent to purge", 
					"event_id", evt.ID, 
					"created_at", time.Unix(evt.CreatedAt, 0).Format(time.RFC3339))
				stats["kept"]++
				totalKept++
				continue
			}

			// Check whitelist status if configured
			if cfg.ExcludeWhitelisted && config.IsPubKeyWhitelisted(evt.PubKey, true) {
				purgeLog().Debug("Skipping whitelisted pubkey", 
					"event_id", evt.ID, 
					"pubkey", evt.PubKey)
				stats["kept"]++
				totalKept++
				continue
			}

			// Check category purge status
			category := utils.DetermineEventCategory(evt.Kind)
			if purge, exists := cfg.PurgeByCategory[category]; !exists || !purge {
				purgeLog().Debug("Skipping excluded category", 
					"event_id", evt.ID, 
					"category", category, 
					"kind", evt.Kind)
				stats["kept"]++
				totalKept++
				continue
			}

			// Delete the event
			_, err = collection.DeleteOne(context.TODO(), bson.M{"id": evt.ID})
			if err != nil {
				purgeLog().Error("Error purging event", 
					"event_id", evt.ID, 
					"collection", collectionName, 
					"error", err)
				stats["kept"]++
				totalKept++
				stats["errors"]++
			} else {
				purgeLog().Debug("Successfully purged event", 
					"event_id", evt.ID, 
					"kind", evt.Kind, 
					"category", category)
				stats["purged"]++
				totalPurged++
			}
		}

		// Update collection stats in map
		collectionStats[collectionName] = stats

		// Log per-collection results
		purgeLog().Info("Collection purge completed", 
			"collection", collectionName,
			"purged", stats["purged"],
			"kept", stats["kept"],
			"errors", stats["errors"])
	}

	// Log overall results
	purgeLog().Info("Purging completed", 
		"total_purged", totalPurged, 
		"total_kept", totalKept,
		"collections_processed", len(collectionsToPurge))
}

// getAllEventCollections returns a list of all event collections if purging all kinds.
func getAllEventCollections(client *mongo.Client) []string {
	var collections []string
	dbName := GetDatabaseName()

	collectionNames, err := client.Database(dbName).ListCollectionNames(context.TODO(), bson.M{})
	if err != nil {
		purgeLog().Error("Error listing collection names", "error", err)
		return collections
	}

	for _, name := range collectionNames {
		if strings.HasPrefix(name, "event-kind") {
			collections = append(collections, name)
		}
	}
	
	purgeLog().Debug("Found event collections", 
		"count", len(collections), 
		"collections", collections)
		
	return collections
}

// ScheduleEventPurging runs the event purging at a configurable interval.
func ScheduleEventPurging(cfg *cfgTypes.ServerConfig) {
    if !cfg.EventPurge.Enabled {
        purgeLog().Info("Event purging is disabled in configuration")
        return
    }

    purgeInterval := time.Duration(cfg.EventPurge.PurgeIntervalMinutes) * time.Minute
    purgeLog().Info("Starting scheduled event purging", 
        "interval_minutes", cfg.EventPurge.PurgeIntervalMinutes,
        "keep_hours", cfg.EventPurge.KeepIntervalHours,
        "disable_initial_purge", cfg.EventPurge.DisableAtStartup)

    ticker := time.NewTicker(purgeInterval)
    defer ticker.Stop()

    // Run initial purge if not disabled
    if !cfg.EventPurge.DisableAtStartup {
        purgeLog().Info("Running initial purge at startup")
        PurgeOldEvents(&cfg.EventPurge)
    } else {
        purgeLog().Info("Initial purge at startup is disabled")
    }

    for range ticker.C {
        purgeLog().Info("Running scheduled purge")
        PurgeOldEvents(&cfg.EventPurge)
        purgeLog().Info("Scheduled purging completed")
    }
}