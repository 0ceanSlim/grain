package mongo

import (
	"context"
	"fmt"
	"log/slog"

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
	var results []relay.Event
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
					
					// Create a query that matches events with tags where:
					// 1. The first element is the tag name (e.g., "e")
					// 2. The second element is in the list of values
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
			filterBson["created_at"] = bson.M{"$gte": *filter.Since}
		}
		if filter.Until != nil {
			if filterBson["created_at"] == nil {
				filterBson["created_at"] = bson.M{"$lte": *filter.Until}
			} else {
				filterBson["created_at"].(bson.M)["$lte"] = *filter.Until
			}
		}

		combinedFilters = append(combinedFilters, filterBson)
	}

	// Combine all filter conditions using the $or operator
	query := bson.M{}
	if len(combinedFilters) > 0 {
		query["$or"] = combinedFilters
	}

	// Apply sorting by creation date (descending)
	opts := options.Find().SetSort(bson.D{
		{Key: "created_at", Value: -1},
		{Key: "id", Value: 1}, // For events with same created_at, sort by ID
	})

	// Get limit from filters or use implicit limit
	var queryLimit int64 = -1 // Default: no limit
	implicitLimit := config.GetConfig().Server.ImplicitReqLimit

	// First check if any filter has a limit
	var lowestExplicitLimit *int
	for _, filter := range filters {
		if filter.Limit != nil {
			if lowestExplicitLimit == nil || *filter.Limit < *lowestExplicitLimit {
				lowestExplicitLimit = filter.Limit
			}
		}
	}

	// Apply the appropriate limit
	if lowestExplicitLimit != nil {
		queryLimit = int64(*lowestExplicitLimit)
		queryLog().Debug("Using explicit limit from filter", "limit", queryLimit)
	} else if implicitLimit > 0 {
		queryLimit = int64(implicitLimit)
		queryLog().Info("No explicit limit specified, applying implicit limit", 
			"implicit_limit", implicitLimit,
			"database", databaseName)
	} else {
		queryLog().Warn("No limit specified and no implicit limit configured", 
			"database", databaseName,
			"query_filters", len(combinedFilters))
	}

	if queryLimit > 0 {
		opts.SetLimit(queryLimit) // Apply the limit
	}

	// If no kinds are specified in any filter, query all collections
	var collections []string
	if len(filters) > 0 && len(filters[0].Kinds) == 0 {
		collections, _ = client.Database(databaseName).ListCollectionNames(context.TODO(), bson.D{})
		queryLog().Debug("No kinds specified, querying all collections", 
			"collection_count", len(collections))
	} else {
		// Collect all kinds from filters and query those collections
		kindsMap := make(map[int]bool)
		for _, filter := range filters {
			for _, kind := range filter.Kinds {
				kindsMap[kind] = true
			}
		}

		// Construct collection names based on kinds
		for kind := range kindsMap {
			collectionName := fmt.Sprintf("event-kind%d", kind)
			collections = append(collections, collectionName)
		}
		queryLog().Debug("Querying specific kind collections", 
			"kind_count", len(kindsMap),
			"collection_count", len(collections))
	}

	totalEvents := 0
	remainingLimit := queryLimit // Store the original limit

	// Query each collection
	for _, collectionName := range collections {
		// If we've already reached the limit, stop querying
		if remainingLimit == 0 && queryLimit > 0 {
			break
		}
		
		// Adjust the limit for this collection query
		if remainingLimit > 0 {
			opts.SetLimit(remainingLimit)
		}
		
		collection := client.Database(databaseName).Collection(collectionName)
		cursor, err := collection.Find(context.TODO(), query, opts)
		if err != nil {
			queryLog().Error("Error querying collection", 
				"collection", collectionName, 
				"error", err)
			return nil, fmt.Errorf("error querying collection %s: %v", collectionName, err)
		}
		defer cursor.Close(context.TODO())

		collectionEvents := 0
		for cursor.Next(context.TODO()) {
			var event relay.Event
			if err := cursor.Decode(&event); err != nil {
				queryLog().Error("Error decoding event", 
					"collection", collectionName, 
					"error", err)
				return nil, fmt.Errorf("error decoding event from collection %s: %v", collectionName, err)
			}
			results = append(results, event)
			collectionEvents++
			
			// Stop if we've reached the limit
			if queryLimit > 0 && len(results) >= int(queryLimit) {
				break
			}
		}
		
		// Update remaining limit
		if remainingLimit > 0 {
			remainingLimit -= int64(collectionEvents)
			if remainingLimit < 0 {
				remainingLimit = 0
			}
		}
		
		totalEvents += collectionEvents

		// Handle cursor errors
		if err := cursor.Err(); err != nil {
			queryLog().Error("Cursor error", 
				"collection", collectionName, 
				"error", err)
			return nil, fmt.Errorf("cursor error in collection %s: %v", collectionName, err)
		}
		
		queryLog().Debug("Collection query complete", 
			"collection", collectionName,
			"events_found", collectionEvents,
			"remaining_limit", remainingLimit,
			"total_events_so_far", len(results))
	}

	queryLog().Info("Query completed", 
		"total_collections", len(collections),
		"total_events", totalEvents,
		"limit_type", func() string {
			if lowestExplicitLimit != nil {
				return "explicit"
			} else if queryLimit > 0 {
				return "implicit"
			}
			return "none"
		}(),
		"limit_value", queryLimit)

	return results, nil
}