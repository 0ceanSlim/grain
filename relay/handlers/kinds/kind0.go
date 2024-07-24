package kinds

import (
	"context"
	"fmt"
	relay "grain/relay/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/websocket"
)

func HandleKind0(ctx context.Context, evt relay.Event, collection *mongo.Collection, ws *websocket.Conn) error {
	// Find the existing event with the same pubkey
	filter := bson.M{"pubkey": evt.PubKey}
	var existingEvent relay.Event
	err := collection.FindOne(ctx, filter).Decode(&existingEvent)
	if err != nil && err != mongo.ErrNoDocuments {
		return fmt.Errorf("error finding existing event: %v", err)
	}

	// If an existing event is found, compare the created_at times
	if err != mongo.ErrNoDocuments {
		if existingEvent.CreatedAt >= evt.CreatedAt {
			// If the existing event is newer or the same, respond with a NOTICE
			sendNotice(ws, evt.PubKey, "relay already has a newer kind 0 event for this pubkey")
			return nil
		}
	}

	// Replace the existing event if it has the same pubkey
	update := bson.M{
		"$set": bson.M{
			"id":         evt.ID,
			"created_at": evt.CreatedAt,
			"kind":       evt.Kind,
			"tags":       evt.Tags,
			"content":    evt.Content,
			"sig":        evt.Sig,
		},
	}

	opts := options.Update().SetUpsert(true) // Insert if not found
	_, err = collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("error updating/inserting event kind 0 into MongoDB: %v", err)
	}

	fmt.Println("Upserted event kind 0 into MongoDB:", evt.ID)
	return nil
}