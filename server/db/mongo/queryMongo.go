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

// buildMongoFilters converts nostr filters to MongoDB filters
func buildMongoFilters(filters []nostr.Filter) []bson.M {
	var combinedFilters []bson.M

	for _, filter := range filters {
		filterBson := buildSingleMongoFilter(filter)
		combinedFilters = append(combinedFilters, filterBson)
	}

	return combinedFilters
}

// buildSingleMongoFilter converts a single nostr filter to MongoDB filter
func buildSingleMongoFilter(filter nostr.Filter) bson.M {
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

	buildTagFilter(filterBson, filter.Tags)
	buildTimeFilters(filterBson, filter.Since, filter.Until)

	return filterBson
}

// buildTagFilter adds tag filtering to the MongoDB filter
// Properly handles multiple tag filters using $and instead of overwriting
func buildTagFilter(filterBson bson.M, tags map[string][]string) {
	if len(tags) == 0 {
		return
	}

	var tagConditions []bson.M

	for key, values := range tags {
		if len(values) > 0 && len(key) > 0 {
			tagKey := strings.TrimPrefix(key, "#")
			tagCondition := bson.M{
				"tags": bson.M{
					"$elemMatch": bson.M{
						"0": tagKey,
						"1": bson.M{"$in": values},
					},
				},
			}
			tagConditions = append(tagConditions, tagCondition)

			log.MongoQuery().Debug("Added tag filter condition",
				"tag_key", tagKey,
				"values", values,
				"condition_count", len(tagConditions))
		}
	}

	// If we have tag conditions, add them to the filter
	if len(tagConditions) > 0 {
		if len(tagConditions) == 1 {
			// Single tag filter - merge directly
			for k, v := range tagConditions[0] {
				filterBson[k] = v
			}
			log.MongoQuery().Debug("Applied single tag filter")
		} else {
			// Multiple tag filters - use $and to combine them
			filterBson["$and"] = tagConditions
			log.MongoQuery().Debug("Applied multiple tag filters using $and",
				"filter_count", len(tagConditions))
		}
	}
}

// buildTimeFilters adds time-based filtering to the MongoDB filter
func buildTimeFilters(filterBson bson.M, since, until *time.Time) {
	if since != nil {
		filterBson["created_at"] = bson.M{"$gte": since.Unix()}
	}
	if until != nil {
		if filterBson["created_at"] == nil {
			filterBson["created_at"] = bson.M{"$lte": until.Unix()}
		} else {
			filterBson["created_at"].(bson.M)["$lte"] = until.Unix()
		}
	}
}

// determineEffectiveLimit calculates the appropriate limit to apply
func determineEffectiveLimit(filters []nostr.Filter) int64 {
	implicitLimit := config.GetConfig().Server.ImplicitReqLimit

	// Find the lowest explicit limit
	var lowestExplicitLimit *int
	for _, filter := range filters {
		if filter.Limit != nil {
			if lowestExplicitLimit == nil || *filter.Limit < *lowestExplicitLimit {
				lowestExplicitLimit = filter.Limit
			}
		}
	}

	// Apply limit logic
	if implicitLimit > 0 {
		if lowestExplicitLimit != nil {
			if *lowestExplicitLimit < implicitLimit {
				log.MongoQuery().Debug("Using explicit limit (under implicit cap)",
					"explicit_limit", *lowestExplicitLimit,
					"implicit_cap", implicitLimit)
				return int64(*lowestExplicitLimit)
			} else {
				log.MongoQuery().Debug("Capping explicit limit to implicit maximum",
					"requested_limit", *lowestExplicitLimit,
					"applied_limit", implicitLimit)
				return int64(implicitLimit)
			}
		} else {
			log.MongoQuery().Debug("Using implicit limit (no explicit limit provided)",
				"limit", implicitLimit)
			return int64(implicitLimit)
		}
	} else {
		if lowestExplicitLimit != nil {
			log.MongoQuery().Debug("Using explicit limit (no implicit cap configured)",
				"limit", *lowestExplicitLimit)
			return int64(*lowestExplicitLimit)
		} else {
			log.MongoQuery().Warn("No limits configured, using default", "default_limit", 1000)
			return 1000
		}
	}
}

// buildQueryFromFilters creates the final MongoDB query from combined filters
func buildQueryFromFilters(combinedFilters []bson.M) bson.M {
	query := bson.M{}

	if len(combinedFilters) == 0 {
		return query
	}

	// Check if any filters have actual content
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

	return query
}

// determineTargetCollections identifies which collections to query based on kind filters
func determineTargetCollections(filters []nostr.Filter, client *mongo.Client, databaseName string) ([]string, bool, error) {
	kindsMap := make(map[int]bool)
	hasKindFilters := false

	// Extract kinds from filters
	for _, filter := range filters {
		if len(filter.Kinds) > 0 {
			hasKindFilters = true
			for _, kind := range filter.Kinds {
				kindsMap[kind] = true
			}
		}
	}

	if !hasKindFilters {
		// Get all event collections for cross-collection query
		allCollections, err := client.Database(databaseName).ListCollectionNames(context.TODO(), bson.D{})
		if err != nil {
			log.MongoQuery().Error("Failed to list collections", "error", err)
			return nil, false, fmt.Errorf("error listing collections: %v", err)
		}

		var collections []string
		for _, name := range allCollections {
			if strings.HasPrefix(name, "event-kind") {
				collections = append(collections, name)
			}
		}
		return collections, false, nil
	} else {
		// Build kind-specific collections
		var collections []string
		for kind := range kindsMap {
			collectionName := fmt.Sprintf("event-kind%d", kind)
			collections = append(collections, collectionName)
		}
		return collections, true, nil
	}
}

// QueryEvents queries events from the MongoDB collection(s) based on filters
func QueryEvents(filters []nostr.Filter, client *mongo.Client, databaseName string) ([]nostr.Event, error) {
	combinedFilters := buildMongoFilters(filters)
	query := buildQueryFromFilters(combinedFilters)
	effectiveLimit := determineEffectiveLimit(filters)

	log.MongoQuery().Debug("Built MongoDB query",
		"filter_count", len(filters),
		"combined_filters", len(combinedFilters),
		"effective_limit", effectiveLimit,
		"query", query)

	collections, hasKindFilters, err := determineTargetCollections(filters, client, databaseName)
	if err != nil {
		return nil, err
	}

	if hasKindFilters {
		return querySpecificKinds(client, databaseName, collections, query, effectiveLimit)
	} else {
		return queryAcrossAllCollections(client, databaseName, collections, query, effectiveLimit)
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

		opts := options.Find().SetSort(bson.D{
			{Key: "created_at", Value: -1},
			{Key: "id", Value: 1},
		})

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

		kindStr := strings.TrimPrefix(collectionName, "event-kind")
		kind, _ := strconv.Atoi(kindStr)

		log.MongoQuery().Debug("Kind collection query complete",
			"collection", collectionName,
			"kind", kind,
			"events_found", len(collectionEvents),
			"limit_applied", limit)
	}

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
