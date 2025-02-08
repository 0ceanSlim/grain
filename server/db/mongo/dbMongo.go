package mongo

import (
	"context"
	"fmt"
	"strings"

	config "grain/config/types"

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

func InitDB(cfg *config.ServerConfig) (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(cfg.MongoDB.URI)
	var err error
	client, err = mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, err
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		return nil, err
	}
	fmt.Println("Connected to MongoDB!")

	// ✅ Store database name globally
	databaseName = cfg.MongoDB.Database

	// ✅ Ensure indexes on all collections
	err = EnsureIndexes(client, databaseName)
	if err != nil {
		fmt.Println("Error ensuring indexes:", err)
	}

	return client, nil
}

// GetDatabaseName returns the database name from config
func GetDatabaseName() string {
	return databaseName
}

func GetCollection(kind int) *mongo.Collection {
	if collection, exists := collections[kind]; exists {
		return collection
	}
	client := GetClient()
	collectionName := fmt.Sprintf("event-kind%d", kind)
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
				fmt.Printf("Failed to create index on %s: %v\n", collectionName, err)
			}
		}
	}

	return collection
}

// Disconnect from MongoDB
func DisconnectDB(client *mongo.Client) {
	err := client.Disconnect(context.TODO())
	if err != nil {
		fmt.Println("Error disconnecting from MongoDB:", err)
	}
	fmt.Println("Disconnected from MongoDB!")
}

func EnsureIndexes(client *mongo.Client, databaseName string) error {
	collections, err := client.Database(databaseName).ListCollectionNames(context.TODO(), bson.D{})
	if err != nil {
		return fmt.Errorf("error listing collections: %v", err)
	}

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

	for _, collectionName := range collections {
		collection := client.Database(databaseName).Collection(collectionName)
		for _, index := range indexes {
			_, err := collection.Indexes().CreateOne(context.TODO(), index)
			if err != nil {
				if !strings.Contains(err.Error(), "IndexKeySpecsConflict") && !strings.Contains(err.Error(), "already exists") {
					fmt.Printf("Error creating index for collection %s: %v\n", collectionName, err)
				}
				continue
			}
		}
		fmt.Printf("Indexes created successfully for collection: %s\n", collectionName)
	}

	return nil
}
