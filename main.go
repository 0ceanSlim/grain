package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/websocket"
)

// Event represents the structure of the incoming events
type Event struct {
	CreatedAt int64    `json:"created_at"`
	Kind      int      `json:"kind"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags"`
	PubKey    string   `json:"pubkey"`
	ID        string   `json:"id"`
	Sig       string   `json:"sig"`
}

// Database client and collections
var client *mongo.Client
var eventKind0Collection *mongo.Collection
var eventKind1Collection *mongo.Collection

func handler(ws *websocket.Conn) {
	var msg string
	for {
		err := websocket.Message.Receive(ws, &msg)
		if err != nil {
			fmt.Println("Error receiving message:", err)
			return
		}
		fmt.Println("Received message:", msg)

		// Parse the received message
		var event []interface{}
		err = json.Unmarshal([]byte(msg), &event)
		if err != nil {
			fmt.Println("Error parsing message:", err)
			return
		}

		if len(event) < 2 || event[0] != "EVENT" {
			fmt.Println("Invalid event format")
			continue
		}

		// Convert the event map to an Event struct
		eventData, ok := event[1].(map[string]interface{})
		if !ok {
			fmt.Println("Invalid event data format")
			continue
		}
		eventBytes, err := json.Marshal(eventData)
		if err != nil {
			fmt.Println("Error marshaling event data:", err)
			continue
		}

		var evt Event
		err = json.Unmarshal(eventBytes, &evt)
		if err != nil {
			fmt.Println("Error unmarshaling event data:", err)
			continue
		}

		// Store the event in the appropriate MongoDB collection
		var collection *mongo.Collection
		switch evt.Kind {
		case 0:
			collection = eventKind0Collection
		case 1:
			collection = eventKind1Collection
		default:
			fmt.Println("Unknown event kind:", evt.Kind)
			continue
		}

		_, err = collection.InsertOne(context.TODO(), evt)
		if err != nil {
			fmt.Println("Error inserting event into MongoDB:", err)
			continue
		}

		fmt.Println("Inserted event into MongoDB:", evt.ID)

		err = websocket.Message.Send(ws, "Echo: "+msg)
		if err != nil {
			fmt.Println("Error sending message:", err)
			return
		}
	}
}

func main() {
	// Initialize MongoDB client
	var err error
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017/")
	client, err = mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Check the connection
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to MongoDB!")

	// Initialize collections
	eventKind0Collection = client.Database("grain").Collection("event-kind0")
	eventKind1Collection = client.Database("grain").Collection("event-kind1")

	// Ensure collections exist by creating an index (which will implicitly create the collections)
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err = eventKind0Collection.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		log.Fatal("Failed to create index on event-kind0: ", err)
	}
	_, err = eventKind1Collection.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		log.Fatal("Failed to create index on event-kind1: ", err)
	}

	// Start WebSocket server
	http.Handle("/", websocket.Handler(handler))
	fmt.Println("WebSocket server started on :8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}

	// Disconnect MongoDB client on exit
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()
}
