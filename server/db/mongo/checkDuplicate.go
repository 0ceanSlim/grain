package mongo

import (
	"context"
	"errors"
	"fmt"
	nostr "grain/server/types" // Adjust import path as needed

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// CheckDuplicateEvent checks if an event with the same ID already exists in the collection.
func CheckDuplicateEvent(ctx context.Context, evt nostr.Event) (bool, error) {
	collection := GetCollection(evt.Kind)
	filter := bson.M{"id": evt.ID}

	var existingEvent nostr.Event
	err := collection.FindOne(ctx, filter).Decode(&existingEvent)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil // No duplicate found
		}
		return false, fmt.Errorf("error checking for duplicate event: %v", err)
	}
	return true, nil // Duplicate found
}
