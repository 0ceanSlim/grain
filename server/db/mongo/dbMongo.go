package mongo

import (
	"context"
	"fmt"
	"strings"

	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var client *mongo.Client
var collections = make(map[int]*mongo.Collection)

// GetClient returns the MongoDB client
func GetClient() *mongo.Client {
	return client
}

var databaseName string // Store the database name globally

func InitDB(cfg *cfgType.ServerConfig) (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(cfg.MongoDB.URI)
	var err error

	log.Mongo().Info("Connecting to MongoDB",
		"uri", cfg.MongoDB.URI,
		"database", cfg.MongoDB.Database)

	client, err = mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Mongo().Error("Failed to connect to MongoDB",
			"uri", cfg.MongoDB.URI,
			"error", err)
		return nil, err
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Mongo().Error("Failed to ping MongoDB", "error", err)
		return nil, err
	}

	log.Mongo().Info("Connected to MongoDB successfully")

	// Store database name globally
	databaseName = cfg.MongoDB.Database

	// Ensure indexes on all collections
	err = EnsureIndexes(client, databaseName)
	if err != nil {
		log.Mongo().Warn("Error ensuring indexes", "error", err)
	}

	return client, nil
}

// GetDatabaseName returns the database name from config
func GetDatabaseName() string {
	return databaseName
}

func GetCollection(kind int) *mongo.Collection {
	collectionName := fmt.Sprintf("event-kind%d", kind)

	if collection, exists := collections[kind]; exists {
		return collection
	}

	log.Mongo().Debug("Creating new collection reference",
		"kind", kind,
		"collection", collectionName)

	client := GetClient()
	collection := client.Database(GetDatabaseName()).Collection(collectionName)
	collections[kind] = collection

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "id", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("unique_id_index"),
		},
		{
			Keys:    bson.D{{Key: "pubkey", Value: 1}},
			Options: options.Index().SetName("pubkey_index"),
		},
		{
			Keys:    bson.D{{Key: "kind", Value: 1}},
			Options: options.Index().SetName("kind_index"),
		},
		{
			Keys:    bson.D{{Key: "created_at", Value: -1}},
			Options: options.Index().SetName("created_at_index"),
		},
	}

	for _, index := range indexes {
		_, err := collection.Indexes().CreateOne(context.TODO(), index)
		if err != nil {
			if !strings.Contains(err.Error(), "IndexKeySpecsConflict") && !strings.Contains(err.Error(), "already exists") {
				log.Mongo().Error("Failed to create index",
					"collection", collectionName,
					"key", index.Keys,
					"error", err)
			}
		}
	}

	log.Mongo().Debug("Collection ready with indexes",
		"kind", kind,
		"collection", collectionName)

	return collection
}

// Disconnect from MongoDB
func DisconnectDB(client *mongo.Client) {
	if client == nil {
		log.Mongo().Warn("Attempted to disconnect nil MongoDB client")
		return
	}

	err := client.Disconnect(context.TODO())
	if err != nil {
		log.Mongo().Error("Error disconnecting from MongoDB", "error", err)
	} else {
		log.Mongo().Info("Disconnected from MongoDB successfully")
	}
}

func EnsureIndexes(client *mongo.Client, databaseName string) error {
	log.Mongo().Info("Ensuring indexes for all collections", "database", databaseName)

	collections, err := client.Database(databaseName).ListCollectionNames(context.TODO(), bson.D{})
	if err != nil {
		log.Mongo().Error("Error listing collections", "error", err)
		return fmt.Errorf("error listing collections: %v", err)
	}

	log.Mongo().Debug("Found collections", "count", len(collections))

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.M{"id": 1},
			Options: options.Index().SetUnique(true).SetName("unique_id_index"),
		},
		{
			Keys:    bson.M{"pubkey": 1},
			Options: options.Index().SetName("pubkey_index"),
		},
		{
			Keys:    bson.M{"kind": 1},
			Options: options.Index().SetName("kind_index"),
		},
		{
			Keys:    bson.M{"created_at": -1},
			Options: options.Index().SetName("created_at_index"),
		},
	}

	indexStats := map[string]int{
		"processed": 0,
		"skipped":   0,
		"errors":    0,
	}

	for _, collectionName := range collections {
		collection := client.Database(databaseName).Collection(collectionName)
		indexStats["processed"]++

		for _, index := range indexes {
			_, err := collection.Indexes().CreateOne(context.TODO(), index)
			if err != nil {
				if strings.Contains(err.Error(), "IndexKeySpecsConflict") ||
					strings.Contains(err.Error(), "already exists") {
					indexStats["skipped"]++
				} else {
					indexStats["errors"]++
					log.Mongo().Error("Error creating index",
						"collection", collectionName,
						"index", index.Keys,
						"error", err)
				}
				continue
			}

			log.Mongo().Debug("Created index",
				"collection", collectionName,
				"index", index.Keys)
		}
	}

	log.Mongo().Info("Index creation completed",
		"collections_processed", indexStats["processed"],
		"indexes_skipped", indexStats["skipped"],
		"errors", indexStats["errors"])

	return nil
}
