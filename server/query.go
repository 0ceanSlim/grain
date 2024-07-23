package server

import (
	"context"
	"fmt"

	server "grain/server/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// QueryEvents queries events from the MongoDB collection based on filters
func QueryEvents(filters []server.Filter, client *mongo.Client, databaseName, collectionName string) ([]server.Event, error) {
	collection := client.Database(databaseName).Collection(collectionName)

	var results []server.Event

	for _, filter := range filters {
		filterBson := bson.M{}

		if len(filter.IDs) > 0 {
			filterBson["_id"] = bson.M{"$in": filter.IDs}
		}
		if len(filter.Authors) > 0 {
			filterBson["author"] = bson.M{"$in": filter.Authors}
		}
		if len(filter.Kinds) > 0 {
			filterBson["kind"] = bson.M{"$in": filter.Kinds}
		}
		if filter.Tags != nil {
			for key, values := range filter.Tags {
				if len(values) > 0 {
					filterBson[key] = bson.M{"$in": values}
				}
			}
		}
		if filter.Since != nil {
			filterBson["created_at"] = bson.M{"$gte": *filter.Since}
		}
		if filter.Until != nil {
			filterBson["created_at"] = bson.M{"$lte": *filter.Until}
		}

		opts := options.Find()
		if filter.Limit != nil {
			opts.SetLimit(int64(*filter.Limit))
		}

		cursor, err := collection.Find(context.TODO(), filterBson, opts)
		if err != nil {
			return nil, fmt.Errorf("error querying events: %v", err)
		}
		defer cursor.Close(context.TODO())

		for cursor.Next(context.TODO()) {
			var event server.Event
			if err := cursor.Decode(&event); err != nil {
				return nil, fmt.Errorf("error decoding event: %v", err)
			}
			results = append(results, event)
		}
		if err := cursor.Err(); err != nil {
			return nil, fmt.Errorf("cursor error: %v", err)
		}
	}

	return results, nil
}
