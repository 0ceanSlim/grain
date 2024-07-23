package events

import (
	"context"
	"fmt"

	server "grain/server/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func HandleKind0(ctx context.Context, evt server.Event, collection *mongo.Collection) error {
	// Replace the existing event if it has the same pubkey
	filter := bson.M{"pubkey": evt.PubKey}
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
	_, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("Error updating/inserting event kind 0 into MongoDB: %v", err)
	}

	fmt.Println("Upserted event kind 0 into MongoDB:", evt.ID)
	return nil
}
