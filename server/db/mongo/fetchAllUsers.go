package mongo

import (
	"context"
	"log"
	"time"

	configTypes "grain/config/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// getAllAuthorsFromRelay fetches all unique authors from MongoDB.
func GetAllAuthorsFromRelay(cfg *configTypes.ServerConfig) []string {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB.URI))
    if err != nil {
        log.Printf("Failed to connect to MongoDB: %v", err)
        return nil
    }
    defer client.Disconnect(ctx)

	db := client.Database(cfg.MongoDB.Database)
	collectionNames, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		log.Printf("Failed to list collections: %v", err)
		return nil
	}

	pubkeySet := make(map[string]struct{})

	for _, collectionName := range collectionNames {
		collection := db.Collection(collectionName)
		cursor, err := collection.Distinct(ctx, "pubkey", bson.M{})
		if err != nil {
			log.Printf("Failed to fetch distinct pubkeys from %s: %v", collectionName, err)
			continue
		}

		for _, pubkey := range cursor {
			if pk, ok := pubkey.(string); ok {
				pubkeySet[pk] = struct{}{}
			}
		}
	}

	authors := make([]string, 0, len(pubkeySet))
	for pubkey := range pubkeySet {
		authors = append(authors, pubkey)
	}

	return authors
}
