package mongo

import (
	"context"
	"strings"
	"time"

	configTypes "github.com/0ceanslim/grain/config/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetAllAuthorsFromRelay fetches all unique authors from MongoDB.
func GetAllAuthorsFromRelay(cfg *configTypes.ServerConfig) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoLog.Debug("Fetching all unique authors from relay")

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoDB.URI))
	if err != nil {
		mongoLog.Error("Failed to connect to MongoDB", 
			"uri", cfg.MongoDB.URI, 
			"error", err)
		return nil
	}
	defer client.Disconnect(ctx)

	db := client.Database(cfg.MongoDB.Database)
	collectionNames, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		mongoLog.Error("Failed to list collections", 
			"database", cfg.MongoDB.Database, 
			"error", err)
		return nil
	}

	mongoLog.Debug("Retrieved collection names", 
		"database", cfg.MongoDB.Database, 
		"collection_count", len(collectionNames))

	pubkeySet := make(map[string]struct{})
	collectionStats := make(map[string]int)

	for _, collectionName := range collectionNames {
		// Skip non-event collections
		if !strings.HasPrefix(collectionName, "event-kind") {
			continue
		}

		collection := db.Collection(collectionName)
		cursor, err := collection.Distinct(ctx, "pubkey", bson.M{})
		if err != nil {
			mongoLog.Error("Failed to fetch distinct pubkeys",
				"collection", collectionName,
				"error", err)
			continue
		}

		// Count pubkeys found in this collection
		pubkeysInCollection := 0
		
		for _, pubkey := range cursor {
			if pk, ok := pubkey.(string); ok {
				pubkeySet[pk] = struct{}{}
				pubkeysInCollection++
			}
		}
		
		collectionStats[collectionName] = pubkeysInCollection
		
		mongoLog.Debug("Processed collection",
			"collection", collectionName,
			"pubkeys_found", pubkeysInCollection)
	}

	authors := make([]string, 0, len(pubkeySet))
	for pubkey := range pubkeySet {
		authors = append(authors, pubkey)
	}

	mongoLog.Info("Completed authors fetch",
		"total_unique_authors", len(authors),
		"collections_processed", len(collectionStats))

	return authors
}
