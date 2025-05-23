package mongo

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

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

	if lowestExplicitLimit != nil {
		effectiveLimit = int64(*lowestExplicitLimit)
		queryLog().Debug("Using explicit limit", "limit", effectiveLimit)
	} else if implicitLimit > 0 {
		effectiveLimit = int64(implicitLimit)
		queryLog().Debug("Using implicit limit", "limit", effectiveLimit)
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
	queryLog().Debug("Querying across all collections for most recent events", 
		"collection_count", len(collections), 
		"limit", limit)
	
	if limit <= 0 {
		queryLog().Warn("No limit specified for cross-collection query, using default", "default_limit", 1000)
		limit = 1000 // Safety fallback
	}

	// Use MongoDB aggregation to efficiently get the most recent events across collections
	var allResults []relay.Event
	
	// Calculate a reasonable per-collection sample size
	// Get more than we need from each collection to ensure we get the true "most recent"
	sampleSize := limit * 2
	if sampleSize > 10000 {
		sampleSize = 10000 // Cap it to prevent excessive memory usage
	}

	for _, collectionName := range collections {
		collection := client.Database(databaseName).Collection(collectionName)
		
		// Get the most recent events from this collection
		opts := options.Find().
			SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "id", Value: 1}}).
			SetLimit(sampleSize)
		
		cursor, err := collection.Find(context.TODO(), query, opts)
		if err != nil {
			queryLog().Error("Error querying collection", "collection", collectionName, "error", err)
			continue // Skip this collection but continue with others
		}
		
		var collectionEvents []relay.Event
		if err := cursor.All(context.TODO(), &collectionEvents); err != nil {
			queryLog().Error("Error decoding events", "collection", collectionName, "error", err)
			cursor.Close(context.TODO())
			continue
		}
		cursor.Close(context.TODO())
		
		allResults = append(allResults, collectionEvents...)
		
		queryLog().Debug("Sampled collection", 
			"collection", collectionName, 
			"events_found", len(collectionEvents))
	}

	// Sort all results by created_at (descending) and apply final limit
	sort.Slice(allResults, func(i, j int) bool {
		if allResults[i].CreatedAt != allResults[j].CreatedAt {
			return allResults[i].CreatedAt > allResults[j].CreatedAt
		}
		return allResults[i].ID < allResults[j].ID
	})

	// Apply the final limit
	if int64(len(allResults)) > limit {
		allResults = allResults[:limit]
	}

	queryLog().Info("Cross-collection query completed", 
		"collections_queried", len(collections),
		"total_sampled", len(allResults),
		"limit_applied", limit,
		"final_count", len(allResults))

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