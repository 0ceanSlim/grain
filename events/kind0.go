package events

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func HandleEventKind0(ctx context.Context, evt Event, collection *mongo.Collection) error {
	// Perform specific validation for event kind 0
	if !isValidEventKind0(evt) {
		return fmt.Errorf("validation failed for event kind 0: %s", evt.ID)
	}

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

	options := options.Update().SetUpsert(true) // Insert if not found
	_, err := collection.UpdateOne(ctx, filter, update, options)
	if err != nil {
		fmt.Println("Error updating/inserting event kind 0 into MongoDB:", err)
		return err
	}

	fmt.Println("Upserted event kind 0 into MongoDB:", evt.ID)
	return nil
}

func isValidEventKind0(evt Event) bool {
	// Placeholder for actual validation logic
	if evt.Content == "" {
		return false
	}
	return true
}
