package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Event represents a Nostr event as stored in MongoDB.
type Event struct {
	ID        string     `json:"id" bson:"id"`
	PubKey    string     `json:"pubkey" bson:"pubkey"`
	CreatedAt int64      `json:"created_at" bson:"created_at"`
	Kind      int        `json:"kind" bson:"kind"`
	Tags      [][]string `json:"tags" bson:"tags"`
	Content   string     `json:"content" bson:"content"`
	Sig       string     `json:"sig" bson:"sig"`
}

func main() {
	uri := flag.String("uri", "mongodb://localhost:27017", "MongoDB connection string")
	database := flag.String("database", "grain", "MongoDB database name")
	output := flag.String("output", "events.jsonl", "Output JSONL file path")
	flag.Parse()

	fmt.Printf("Connecting to MongoDB at %s...\n", *uri)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(*uri))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to MongoDB: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect(context.Background())

	// Verify connection
	if err := client.Ping(ctx, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to ping MongoDB: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Connected to MongoDB successfully.")

	db := client.Database(*database)

	// List all collections matching event-kind* pattern
	collections, err := db.ListCollectionNames(ctx, bson.M{"name": bson.M{"$regex": "^event-kind"}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list collections: %v\n", err)
		os.Exit(1)
	}

	if len(collections) == 0 {
		fmt.Println("No event-kind* collections found in database.")
		os.Exit(0)
	}

	fmt.Printf("Found %d event collection(s)\n\n", len(collections))

	// Open output file
	file, err := os.Create(*output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	totalEvents := 0
	skippedEvents := 0

	for _, collName := range collections {
		coll := db.Collection(collName)
		collCount := 0

		cursor, err := coll.Find(context.Background(), bson.M{})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [%s] Failed to query: %v (skipping)\n", collName, err)
			continue
		}

		for cursor.Next(context.Background()) {
			// Decode into raw bson to strip _id field
			var raw bson.M
			if err := cursor.Decode(&raw); err != nil {
				fmt.Fprintf(os.Stderr, "  [%s] Failed to decode document: %v (skipping)\n", collName, err)
				skippedEvents++
				continue
			}

			// Remove MongoDB's _id field
			delete(raw, "_id")

			// Re-marshal to our Event struct to ensure clean output
			jsonBytes, err := json.Marshal(raw)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  [%s] Failed to marshal intermediate: %v (skipping)\n", collName, err)
				skippedEvents++
				continue
			}

			var evt Event
			if err := json.Unmarshal(jsonBytes, &evt); err != nil {
				fmt.Fprintf(os.Stderr, "  [%s] Failed to unmarshal to Event: %v (skipping)\n", collName, err)
				skippedEvents++
				continue
			}

			// Validate minimal fields
			if evt.ID == "" || evt.PubKey == "" || evt.Sig == "" {
				fmt.Fprintf(os.Stderr, "  [%s] Skipping event with missing required fields\n", collName)
				skippedEvents++
				continue
			}

			// Ensure tags is never null in JSON output
			if evt.Tags == nil {
				evt.Tags = [][]string{}
			}

			outBytes, err := json.Marshal(evt)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  [%s] Failed to marshal event: %v (skipping)\n", collName, err)
				skippedEvents++
				continue
			}

			file.Write(outBytes)
			file.WriteString("\n")
			collCount++
		}

		if err := cursor.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "  [%s] Cursor error: %v\n", collName, err)
		}
		cursor.Close(context.Background())

		fmt.Printf("  %-30s %d events\n", collName, collCount)
		totalEvents += collCount
	}

	fmt.Printf("\nExport complete: %d events written to %s\n", totalEvents, *output)
	if skippedEvents > 0 {
		fmt.Printf("Skipped %d malformed documents\n", skippedEvents)
	}
}
