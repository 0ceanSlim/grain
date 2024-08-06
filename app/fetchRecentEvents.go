package app

import (
	"context"
	relay "grain/server/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// FetchTopTenRecentEvents queries the database and returns the top ten most recent events.
func FetchTopTenRecentEvents(client *mongo.Client) ([]relay.Event, error) {
	var results []relay.Event

	collections, err := client.Database("grain").ListCollectionNames(context.TODO(), bson.M{})
	if err != nil {
		return nil, err
	}

	for _, collectionName := range collections {
		collection := client.Database("grain").Collection(collectionName)
		filter := bson.D{}
		opts := options.Find().SetSort(bson.D{{Key: "createdat", Value: -1}}).SetLimit(10)

		cursor, err := collection.Find(context.TODO(), filter, opts)
		if err != nil {
			return nil, err
		}
		defer cursor.Close(context.TODO())

		for cursor.Next(context.TODO()) {
			var event relay.Event
			if err := cursor.Decode(&event); err != nil {
				return nil, err
			}
			results = append(results, event)
		}

		if err := cursor.Err(); err != nil {
			return nil, err
		}
	}

	return results, nil
}

