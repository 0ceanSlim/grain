package db

import (
	"context"
	"fmt"

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

// Initialize MongoDB client
func InitDB(uri, database string) (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(uri)
	var err error
	client, err = mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, err
	}

	// Check the connection
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		return nil, err
	}
	fmt.Println("Connected to MongoDB!")

	return client, nil
}

func GetCollection(kind int) *mongo.Collection {
	if collection, exists := collections[kind]; exists {
		return collection
	}
	client := GetClient()
	collectionName := fmt.Sprintf("event-kind%d", kind)
	collection := client.Database("grain").Collection(collectionName)
	collections[kind] = collection
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err := collection.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		fmt.Printf("Failed to create index on %s: %v\n", collectionName, err)
	}
	return collection
}

// Disconnect from MongoDB
func DisconnectDB() {
	if err := client.Disconnect(context.TODO()); err != nil {
		fmt.Println("Error disconnecting from MongoDB:", err)
	}
	fmt.Println("Disconnected from MongoDB!")
}

