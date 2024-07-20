package events

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Event struct {
	ID        string   `json:"id"`
	PubKey    string   `json:"pubkey"`
	CreatedAt int64    `json:"created_at"`
	Kind      int      `json:"kind"`
	Tags      []string `json:"tags"`
	Content   string   `json:"content"`
	Sig       string   `json:"sig"`
}

var eventKind0Collection *mongo.Collection
var eventKind1Collection *mongo.Collection

func InitCollections(client *mongo.Client, eventKind0, eventKind1 string) {
	eventKind0Collection = client.Database("grain").Collection(eventKind0)
	eventKind1Collection = client.Database("grain").Collection(eventKind1)

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}
	_, err := eventKind0Collection.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		fmt.Println("Failed to create index on event-kind0: ", err)
	}
	_, err = eventKind1Collection.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		fmt.Println("Failed to create index on event-kind1: ", err)
	}
}

func HandleEvent(ctx context.Context, evt Event) error {
	var collection *mongo.Collection
	switch evt.Kind {
	case 0:
		collection = eventKind0Collection
	case 1:
		return HandleEventKind1(ctx, evt, eventKind1Collection)
	default:
		fmt.Println("Unknown event kind:", evt.Kind)
		return fmt.Errorf("unknown event kind: %d", evt.Kind)
	}

	_, err := collection.InsertOne(ctx, evt)
	if err != nil {
		fmt.Println("Error inserting event into MongoDB:", err)
		return err
	}

	fmt.Println("Inserted event into MongoDB:", evt.ID)
	return nil
}

func GetCollections() map[string]*mongo.Collection {
	return map[string]*mongo.Collection{
		"eventKind0": eventKind0Collection,
		"eventKind1": eventKind1Collection,
	}
}
