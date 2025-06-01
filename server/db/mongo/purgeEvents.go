package mongo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/0ceanslim/grain/config"
	cfgTypes "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// PurgeOldEventsOptimized removes old events using cached whitelist data for bulk operations
func PurgeOldEventsOptimized(cfg *cfgTypes.EventPurgeConfig) {
	if !cfg.Enabled {
		log.MongoPurge().Debug("Event purging is disabled")
		return
	}

	log.MongoPurge().Info("Starting optimized event purge", 
		"keep_hours", cfg.KeepIntervalHours,
		"exclude_whitelisted", cfg.ExcludeWhitelisted,
		"purge_by_kind_enabled", cfg.PurgeByKindEnabled)

	client := GetClient()
	dbName := GetDatabaseName()

	currentTime := time.Now().Unix()
	cutoff := currentTime - int64(cfg.KeepIntervalHours*3600)
	cutoffTime := time.Unix(cutoff, 0)

	log.MongoPurge().Debug("Purge cutoff calculated", 
		"current_time", time.Unix(currentTime, 0).Format(time.RFC3339),
		"cutoff_time", cutoffTime.Format(time.RFC3339),
		"cutoff_unix", cutoff)

	// Get cached whitelist if exclusion is enabled
	// Use GetWhitelistedPubkeys() which ignores enabled state for purge operations
	var whitelistedPubkeys []string
	if cfg.ExcludeWhitelisted {
		pubkeyCache := config.GetPubkeyCache()
		whitelistedPubkeys = pubkeyCache.GetWhitelistedPubkeys()
		
		log.MongoPurge().Info("Using cached whitelist for purge exclusion", 
			"whitelisted_count", len(whitelistedPubkeys),
			"exclude_whitelisted", cfg.ExcludeWhitelisted)
		
		if len(whitelistedPubkeys) == 0 {
			log.MongoPurge().Warn("No whitelisted pubkeys found in cache, purge will include all pubkeys")
		}
	}

	var collectionsToPurge []string
	totalPurged := 0

	// Determine collections to purge
	if cfg.PurgeByKindEnabled {
		log.MongoPurge().Debug("Using kind-specific purging", "kinds_to_purge", cfg.KindsToPurge)
		for _, kind := range cfg.KindsToPurge {
			collectionName := fmt.Sprintf("event-kind%d", kind)
			collectionsToPurge = append(collectionsToPurge, collectionName)
		}
	} else {
		log.MongoPurge().Debug("Using category-based purging")
		collectionsToPurge = getAllEventCollections(client)
	}

	log.MongoPurge().Info("Identified collections for purging", 
		"collection_count", len(collectionsToPurge),
		"category_purging", cfg.PurgeByCategory)

	// Process each collection with bulk operations
	for _, collectionName := range collectionsToPurge {
		purged := purgeCollectionOptimized(client, dbName, collectionName, cutoff, cfg, whitelistedPubkeys)
		totalPurged += purged
		
		if purged > 0 {
			log.MongoPurge().Info("Collection purge completed", 
				"collection", collectionName,
				"purged", purged)
		} else {
			log.MongoPurge().Debug("No documents purged from collection", 
				"collection", collectionName)
		}
	}

	log.MongoPurge().Info("Optimized purging completed", 
		"total_purged", totalPurged,
		"collections_processed", len(collectionsToPurge))
}

// purgeCollectionOptimized performs bulk deletion on a single collection
func purgeCollectionOptimized(client *mongo.Client, dbName, collectionName string, cutoff int64, cfg *cfgTypes.EventPurgeConfig, whitelistedPubkeys []string) int {
	collection := client.Database(dbName).Collection(collectionName)
	
	// Build base filter for old events
	filter := bson.M{"created_at": bson.M{"$lt": cutoff}}
	
	// Add whitelist exclusion if configured
	if cfg.ExcludeWhitelisted && len(whitelistedPubkeys) > 0 {
		filter["pubkey"] = bson.M{"$nin": whitelistedPubkeys}
		log.MongoPurge().Debug("Added whitelist exclusion to filter", 
			"collection", collectionName,
			"excluded_pubkeys", len(whitelistedPubkeys))
	}
	
	// Add category filtering if needed
	if len(cfg.PurgeByCategory) > 0 {
		// Extract kind from collection name
		kindStr := strings.TrimPrefix(collectionName, "event-kind")
		if kindStr != collectionName { // Valid kind collection
			// Determine category for this kind (reuse existing logic)
			kind := 0
			if _, err := fmt.Sscanf(kindStr, "%d", &kind); err == nil {
				category := utils.DetermineEventCategory(kind)
				
				// Check if this category should be purged
				if purge, exists := cfg.PurgeByCategory[category]; !exists || !purge {
					log.MongoPurge().Debug("Skipping collection due to category exclusion", 
						"collection", collectionName,
						"kind", kind, 
						"category", category)
					return 0
				}
			}
		}
	}
	
	// Count documents that will be deleted (for logging)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		log.MongoPurge().Error("Error counting documents for purge", 
			"collection", collectionName, 
			"error", err)
		return 0
	}
	
	if count == 0 {
		log.MongoPurge().Debug("No documents to purge in collection", 
			"collection", collectionName)
		return 0
	}
	
	log.MongoPurge().Info("Starting bulk delete operation", 
		"collection", collectionName,
		"documents_to_delete", count)
	
	// Perform bulk deletion
	start := time.Now()
	result, err := collection.DeleteMany(ctx, filter)
	duration := time.Since(start)
	
	if err != nil {
		log.MongoPurge().Error("Bulk delete operation failed", 
			"collection", collectionName,
			"error", err)
		return 0
	}
	
	log.MongoPurge().Info("Bulk delete operation completed", 
		"collection", collectionName,
		"deleted_count", result.DeletedCount,
		"expected_count", count,
		"duration_ms", duration.Milliseconds())
	
	return int(result.DeletedCount)
}

// ScheduleEventPurgingOptimized runs the optimized event purging at configurable intervals
func ScheduleEventPurgingOptimized(cfg *cfgTypes.ServerConfig) {
	if !cfg.EventPurge.Enabled {
		log.MongoPurge().Info("Event purging is disabled in configuration")
		return
	}

	purgeInterval := time.Duration(cfg.EventPurge.PurgeIntervalMinutes) * time.Minute
	log.MongoPurge().Info("Starting scheduled optimized event purging", 
		"interval_minutes", cfg.EventPurge.PurgeIntervalMinutes,
		"keep_hours", cfg.EventPurge.KeepIntervalHours,
		"disable_initial_purge", cfg.EventPurge.DisableAtStartup)

	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()

	// Run initial purge if not disabled
	if !cfg.EventPurge.DisableAtStartup {
		log.MongoPurge().Info("Running initial optimized purge at startup")
		PurgeOldEventsOptimized(&cfg.EventPurge)
	} else {
		log.MongoPurge().Info("Initial purge at startup is disabled")
	}

	for range ticker.C {
		log.MongoPurge().Info("Running scheduled optimized purge")
		PurgeOldEventsOptimized(&cfg.EventPurge)
		log.MongoPurge().Info("Scheduled optimized purging completed")
	}
}

// getAllEventCollections returns a list of all event collections if purging all kinds.
func getAllEventCollections(client *mongo.Client) []string {
	var collections []string
	dbName := GetDatabaseName()

	collectionNames, err := client.Database(dbName).ListCollectionNames(context.TODO(), bson.M{})
	if err != nil {
		log.MongoPurge().Error("Error listing collection names", "error", err)
		return collections
	}

	for _, name := range collectionNames {
		if strings.HasPrefix(name, "event-kind") {
			collections = append(collections, name)
		}
	}
	
	log.MongoPurge().Debug("Found event collections", 
		"count", len(collections), 
		"collections", collections)
		
	return collections
}

