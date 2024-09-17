package mongo

import (
	"context"
	"fmt"
	relay "grain/server/types"

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

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})

	// Apply limit if set for initial query
	for _, filter := range filters {
		if filter.Limit != nil {
			opts.SetLimit(int64(*filter.Limit))
		}
	}

	// If no specific kinds are specified, query all collections
	if len(filters[0].Kinds) == 0 {
		collections, err := client.Database(databaseName).ListCollectionNames(context.TODO(), bson.D{})
		if err != nil {
			return nil, fmt.Errorf("error listing collections: %v", err)
		}

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
			if err := cursor.Err(); err != nil {
				return nil, fmt.Errorf("cursor error in collection %s: %v", collectionName, err)
			}
		}
	} else {
		// Query specific collections based on kinds
		for _, kind := range filters[0].Kinds {
			collectionName := fmt.Sprintf("event-kind%d", kind)
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
			if err := cursor.Err(); err != nil {
				return nil, fmt.Errorf("cursor error in collection %s: %v", collectionName, err)
			}
		}
	}

	return results, nil
}
