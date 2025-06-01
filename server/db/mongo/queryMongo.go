package mongo

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/0ceanslim/grain/config"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// QueryEvents queries events from the MongoDB collection(s) based on filters
func QueryEvents(filters []nostr.Filter, client *mongo.Client, databaseName string) ([]nostr.Event, error) {
	var combinedFilters []bson.M

	// Build MongoDB filters for each nostr.Filter
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
				log.MongoQuery().Debug("Using explicit limit (under implicit cap)", 
					"explicit_limit", *lowestExplicitLimit,
					"implicit_cap", implicitLimit)
			} else {
				effectiveLimit = int64(implicitLimit)
				log.MongoQuery().Debug("Capping explicit limit to implicit maximum", 
					"requested_limit", *lowestExplicitLimit,
					"applied_limit", implicitLimit)
			}
		} else {
			// No explicit limit, use implicit limit
			effectiveLimit = int64(implicitLimit)
			log.MongoQuery().Debug("Using implicit limit (no explicit limit provided)", 
				"limit", implicitLimit)
		}
	} else {
		// No implicit limit configured
		if lowestExplicitLimit != nil {
			effectiveLimit = int64(*lowestExplicitLimit)
			log.MongoQuery().Debug("Using explicit limit (no implicit cap configured)", 
				"limit", *lowestExplicitLimit)
		} else {
			// No limits at all - use a sensible default
			effectiveLimit = 1000
			log.MongoQuery().Warn("No limits configured, using default", "default_limit", 1000)
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
			log.MongoQuery().Error("Failed to list collections", "error", err)
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
func queryAcrossAllCollections(client *mongo.Client, databaseName string, collections []string, query bson.M, limit int64) ([]nostr.Event, error) {
	log.MongoQuery().Debug("Starting unified cross-collection query with $unionWith", 
		"collection_count", len(collections), 
		"limit", limit)

	if len(collections) == 0 {
		return []nostr.Event{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	start := time.Now()

	// Build aggregation pipeline with $unionWith for all collections
	pipeline := []bson.M{}

	// Start with match on the first collection
	pipeline = append(pipeline, bson.M{"$match": query})

	// Add $unionWith for all other collections
	for _, collectionName := range collections[1:] {
		pipeline = append(pipeline, bson.M{
			"$unionWith": bson.M{
				"coll": collectionName,
				"pipeline": []bson.M{
					{"$match": query},
				},
			},
		})
	}

	// Sort by created_at (most recent first) and apply limit
	pipeline = append(pipeline, 
		bson.M{"$sort": bson.M{"created_at": -1, "id": 1}},
		bson.M{"$limit": limit},
	)

	log.MongoQuery().Debug("Executing unified aggregation", 
		"pipeline_stages", len(pipeline),
		"collections_to_union", len(collections),
		"base_collection", collections[0])

	// Execute aggregation on the first collection
	collection := client.Database(databaseName).Collection(collections[0])
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		log.MongoQuery().Error("Unified aggregation failed", 
			"error", err,
			"collections", len(collections))
		return nil, fmt.Errorf("unified cross-collection query failed: %v", err)
	}
	defer cursor.Close(ctx)

	var events []nostr.Event
	if err := cursor.All(ctx, &events); err != nil {
		log.MongoQuery().Error("Failed to decode unified results", "error", err)
		return nil, fmt.Errorf("failed to decode unified query results: %v", err)
	}

	duration := time.Since(start)
	log.MongoQuery().Info("Unified cross-collection query completed", 
		"duration_ms", duration.Milliseconds(),
		"collections_unified", len(collections),
		"results", len(events),
		"chronological_order", "guaranteed")

	return events, nil
}

// querySpecificKinds queries specific kind collections with limit per kind
func querySpecificKinds(client *mongo.Client, databaseName string, collections []string, query bson.M, limit int64) ([]nostr.Event, error) {
	log.MongoQuery().Debug("Querying specific kinds", 
		"collection_count", len(collections), 
		"limit_per_kind", limit)

	var allResults []nostr.Event
	
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
			log.MongoQuery().Error("Error querying collection", "collection", collectionName, "error", err)
			continue
		}
		
		var collectionEvents []nostr.Event
		if err := cursor.All(context.TODO(), &collectionEvents); err != nil {
			log.MongoQuery().Error("Error decoding events", "collection", collectionName, "error", err)
			cursor.Close(context.TODO())
			continue
		}
		cursor.Close(context.TODO())
		
		allResults = append(allResults, collectionEvents...)
		
		// Extract kind from collection name for logging
		kindStr := strings.TrimPrefix(collectionName, "event-kind")
		kind, _ := strconv.Atoi(kindStr)
		
		log.MongoQuery().Debug("Kind collection query complete", 
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

	log.MongoQuery().Info("Specific kinds query completed", 
		"kinds_queried", len(collections),
		"total_events", len(allResults),
		"limit_per_kind", limit)

	return allResults, nil
}