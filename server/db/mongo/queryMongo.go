package mongo

import (
	"context"
	"fmt"

	relay "github.com/0ceanslim/grain/server/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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
		if filter.Tags != nil {
			for key, values := range filter.Tags {
				if len(values) > 0 {
					filterBson["tags."+key] = bson.M{"$in": values}
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
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	var queryLimit int64 = -1 // Default: no limit

	for _, filter := range filters {
		if filter.Limit != nil {
			if queryLimit == -1 || int64(*filter.Limit) < queryLimit {
				queryLimit = int64(*filter.Limit)
			}
		}
	}

	if queryLimit > 0 {
		opts.SetLimit(queryLimit) // Apply the lowest limit found
	}

	// If no kinds are specified in any filter, query all collections
	var collections []string
	if len(filters) > 0 && len(filters[0].Kinds) == 0 {
		collections, _ = client.Database(databaseName).ListCollectionNames(context.TODO(), bson.D{})
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
	}

	// Query each collection
	for _, collectionName := range collections {
		collection := client.Database(databaseName).Collection(collectionName)
		cursor, err := collection.Find(context.TODO(), query, opts)
		if err != nil {
			return nil, fmt.Errorf("error querying collection %s: %v", collectionName, err)
		}
		defer cursor.Close(context.TODO())

		for cursor.Next(context.TODO()) {
			var event relay.Event
			if err := cursor.Decode(&event); err != nil {
				return nil, fmt.Errorf("error decoding event from collection %s: %v", collectionName, err)
			}
			results = append(results, event)
		}

		// Handle cursor errors
		if err := cursor.Err(); err != nil {
			return nil, fmt.Errorf("cursor error in collection %s: %v", collectionName, err)
		}
	}

	return results, nil
}
