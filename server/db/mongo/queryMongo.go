package mongo

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/0ceanslim/grain/config"
	relay "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Set the logging component for MongoDB query operations
func queryLog() *slog.Logger {
	return utils.GetLogger("mongo-query")
}

// QueryEvents queries events from the MongoDB collection(s) based on filters
func QueryEvents(filters []relay.Filter, client *mongo.Client, databaseName string) ([]relay.Event, error) {
	var combinedFilters []bson.M

	// Build MongoDB filters for each relay.Filter
	for _, filter := range filters {
		filterBson := bson.M{}

		if len(filter.IDs) > 0 {
			filterBson["id"] = bson.M{"$in": filter.IDs}
		}
		if len(filter.Authors) > 0 {
			filterBson["pubkey"] = bson.M{"$in": filter.Authors}
		}
		if len(filter.Kinds) > 0 {
			filterBson["kind"] = bson.M{"$in": filter.Kinds}
		}
		
		// Tag filtering implementation
		if filter.Tags != nil {
			for key, values := range filter.Tags {
				if len(values) > 0 && len(key) > 0 {
					// Remove the # prefix if present
					tagKey := key
					if tagKey[0] == '#' {
						tagKey = tagKey[1:]
					}
					
					filterBson["tags"] = bson.M{
						"$elemMatch": bson.M{
							"0": tagKey,
							"1": bson.M{"$in": values},
						},
					}
				}
			}
		}
		if filter.Since != nil {
			filterBson["created_at"] = bson.M{"$gte": filter.Since.Unix()}
		}
		if filter.Until != nil {
			if filterBson["created_at"] == nil {
				filterBson["created_at"] = bson.M{"$lte": filter.Until.Unix()}
			} else {
				filterBson["created_at"].(bson.M)["$lte"] = filter.Until.Unix()
			}
		}

		combinedFilters = append(combinedFilters, filterBson)
	}

	// Handle empty filters properly
	query := bson.M{}
	if len(combinedFilters) > 0 {
		hasActualFilters := false
		for _, filter := range combinedFilters {
			if len(filter) > 0 {
				hasActualFilters = true
				break
			}
		}
		
		if hasActualFilters {
			query["$or"] = combinedFilters
		}
	}

	// Determine the limit to apply
	implicitLimit := config.GetConfig().Server.ImplicitReqLimit
	var effectiveLimit int64 = -1

	// Check if any filter has an explicit limit
	var lowestExplicitLimit *int
	for _, filter := range filters {
		if filter.Limit != nil {
			if lowestExplicitLimit == nil || *filter.Limit < *lowestExplicitLimit {
				lowestExplicitLimit = filter.Limit
			}
		}
	}

	// Apply the limit logic: implicit limit is the maximum cap
	if implicitLimit > 0 {
		if lowestExplicitLimit != nil {
			// Use the smaller of explicit limit and implicit limit
			if *lowestExplicitLimit < implicitLimit {
				effectiveLimit = int64(*lowestExplicitLimit)
				queryLog().Debug("Using explicit limit (under implicit cap)", 
					"explicit_limit", *lowestExplicitLimit,
					"implicit_cap", implicitLimit)
			} else {
				effectiveLimit = int64(implicitLimit)
				queryLog().Debug("Capping explicit limit to implicit maximum", 
					"requested_limit", *lowestExplicitLimit,
					"applied_limit", implicitLimit)
			}
		} else {
			// No explicit limit, use implicit limit
			effectiveLimit = int64(implicitLimit)
			queryLog().Debug("Using implicit limit (no explicit limit provided)", 
				"limit", implicitLimit)
		}
	} else {
		// No implicit limit configured
		if lowestExplicitLimit != nil {
			effectiveLimit = int64(*lowestExplicitLimit)
			queryLog().Debug("Using explicit limit (no implicit cap configured)", 
				"limit", *lowestExplicitLimit)
		} else {
			// No limits at all - use a sensible default
			effectiveLimit = 1000
			queryLog().Warn("No limits configured, using default", "default_limit", 1000)
		}
	}

	// Collection selection logic
	var collections []string
	hasKindFilters := false
	kindsMap := make(map[int]bool)
	
	for _, filter := range filters {
		if len(filter.Kinds) > 0 {
			hasKindFilters = true
			for _, kind := range filter.Kinds {
				kindsMap[kind] = true
			}
		}
	}
	
	if !hasKindFilters {
		// No kinds specified - get all event collections for cross-collection query
		allCollections, err := client.Database(databaseName).ListCollectionNames(context.TODO(), bson.D{})
		if err != nil {
			queryLog().Error("Failed to list collections", "error", err)
			return nil, fmt.Errorf("error listing collections: %v", err)
		}
		
		for _, name := range allCollections {
			if len(name) > 10 && name[:10] == "event-kind" {
				collections = append(collections, name)
			}
		}
		
		// For no-kind queries, use MongoDB aggregation for efficiency
		return queryAcrossAllCollections(client, databaseName, collections, query, effectiveLimit)
	} else {
		// Kinds specified - query each kind collection with limit per kind
		for kind := range kindsMap {
			collectionName := fmt.Sprintf("event-kind%d", kind)
			collections = append(collections, collectionName)
		}
		
		return querySpecificKinds(client, databaseName, collections, query, effectiveLimit)
	}
}

// queryAcrossAllCollections efficiently queries the most recent events across all collections
func queryAcrossAllCollections(client *mongo.Client, databaseName string, collections []string, query bson.M, limit int64) ([]relay.Event, error) {
	queryLog().Debug("Starting optimized cross-collection query", 
		"collection_count", len(collections), 
		"limit", limit)
	
	if limit <= 0 {
		queryLog().Warn("No limit specified, using default", "default_limit", 500)
		limit = 500
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// MUCH more conservative sampling strategy
	// For 150 collections and limit=500, we want ~3-5 events per collection max
	conservativeSampleSize := int64(10) // Start very small
	if limit <= 50 {
		conservativeSampleSize = 3
	} else if limit <= 200 {
		conservativeSampleSize = 5
	} else {
		conservativeSampleSize = 10
	}

	queryLog().Debug("Using conservative sampling", 
		"sample_per_collection", conservativeSampleSize,
		"max_total_samples", conservativeSampleSize*int64(len(collections)))

	type collectionResult struct {
		events []relay.Event
		name   string
	}

	resultChan := make(chan collectionResult, len(collections))
	semaphore := make(chan struct{}, 8) // Limit concurrent queries

	// Query collections concurrently with strict limits
	for _, collectionName := range collections {
		go func(name string) {
			semaphore <- struct{}{} // Acquire
			defer func() { <-semaphore }() // Release

			collection := client.Database(databaseName).Collection(name)
			
			// Use aggregation pipeline for efficiency
			pipeline := []bson.M{
				{"$match": query},
				{"$sort": bson.M{"created_at": -1, "id": 1}},
				{"$limit": conservativeSampleSize},
			}

			// Set a short timeout per collection
			collectionCtx, collectionCancel := context.WithTimeout(ctx, 2*time.Second)
			defer collectionCancel()

			cursor, err := collection.Aggregate(collectionCtx, pipeline)
			if err != nil {
				queryLog().Debug("Error querying collection", 
					"collection", name, 
					"error", err)
				resultChan <- collectionResult{name: name}
				return
			}
			defer cursor.Close(collectionCtx)

			var events []relay.Event
			if err := cursor.All(collectionCtx, &events); err != nil {
				queryLog().Debug("Error decoding events", 
					"collection", name, 
					"error", err)
				resultChan <- collectionResult{name: name}
				return
			}

			resultChan <- collectionResult{
				events: events,
				name:   name,
			}
		}(collectionName)
	}

	// Collect results with timeout protection
	var allResults []relay.Event
	collectionsProcessed := 0

	collectLoop:
		for collectionsProcessed < len(collections) {
			select {
			case result := <-resultChan:
				collectionsProcessed++
				if len(result.events) > 0 {
					allResults = append(allResults, result.events...)
					queryLog().Debug("Collected from collection", 
						"collection", result.name,
						"events", len(result.events),
						"total_so_far", len(allResults))
				}
				
				// Early termination if we have way more than needed
				if int64(len(allResults)) >= limit*3 {
					queryLog().Debug("Early termination - sufficient results collected")
					break collectLoop
				}

			case <-ctx.Done():
				queryLog().Warn("Query timeout reached", 
					"processed", collectionsProcessed,
					"total", len(collections))
				break collectLoop
			}
		}

		// Sort all results by created_at (descending)
		sort.Slice(allResults, func(i, j int) bool {
			if allResults[i].CreatedAt != allResults[j].CreatedAt {
				return allResults[i].CreatedAt > allResults[j].CreatedAt
			}
			return allResults[i].ID < allResults[j].ID
		})

		// Apply final limit
		if int64(len(allResults)) > limit {
			allResults = allResults[:limit]
		}

	queryLog().Info("Optimized cross-collection query completed", 
		"collections_processed", collectionsProcessed,
		"total_collections", len(collections),
		"events_collected", len(allResults),
		"final_count", len(allResults),
		"sample_per_collection", conservativeSampleSize)

	return allResults, nil
}

// querySpecificKinds queries specific kind collections with limit per kind
func querySpecificKinds(client *mongo.Client, databaseName string, collections []string, query bson.M, limit int64) ([]relay.Event, error) {
	queryLog().Debug("Querying specific kinds", 
		"collection_count", len(collections), 
		"limit_per_kind", limit)

	var allResults []relay.Event
	
	for _, collectionName := range collections {
		collection := client.Database(databaseName).Collection(collectionName)
		
		// Create fresh options for each collection
		opts := options.Find().SetSort(bson.D{
			{Key: "created_at", Value: -1},
			{Key: "id", Value: 1},
		})
		
		// Apply limit per kind if specified
		if limit > 0 {
			opts.SetLimit(limit)
		}
		
		cursor, err := collection.Find(context.TODO(), query, opts)
		if err != nil {
			queryLog().Error("Error querying collection", "collection", collectionName, "error", err)
			continue
		}
		
		var collectionEvents []relay.Event
		if err := cursor.All(context.TODO(), &collectionEvents); err != nil {
			queryLog().Error("Error decoding events", "collection", collectionName, "error", err)
			cursor.Close(context.TODO())
			continue
		}
		cursor.Close(context.TODO())
		
		allResults = append(allResults, collectionEvents...)
		
		// Extract kind from collection name for logging
		kindStr := strings.TrimPrefix(collectionName, "event-kind")
		kind, _ := strconv.Atoi(kindStr)
		
		queryLog().Debug("Kind collection query complete", 
			"collection", collectionName,
			"kind", kind,
			"events_found", len(collectionEvents),
			"limit_applied", limit)
	}

	// Sort final results by created_at (descending)
	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].CreatedAt != allResults[j].CreatedAt {
			return allResults[i].CreatedAt > allResults[j].CreatedAt
		}
		return allResults[i].ID < allResults[j].ID
	})

	queryLog().Info("Specific kinds query completed", 
		"kinds_queried", len(collections),
		"total_events", len(allResults),
		"limit_per_kind", limit)

	return allResults, nil
}