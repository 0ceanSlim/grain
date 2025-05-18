package mongo

import (
	"context"
	"errors"
	"fmt"

	nostr "github.com/0ceanslim/grain/server/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// CheckDuplicateEvent checks if an event with the same ID already exists in the collection.
func CheckDuplicateEvent(ctx context.Context, evt nostr.Event) (bool, error) {
	collection := GetCollection(evt.Kind)
	filter := bson.M{"id": evt.ID}

	mongoLog.Debug("Checking for duplicate event", 
		"event_id", evt.ID,
		"kind", evt.Kind,
		"collection", fmt.Sprintf("event-kind%d", evt.Kind))

	var existingEvent nostr.Event
	err := collection.FindOne(ctx, filter).Decode(&existingEvent)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			mongoLog.Debug("No duplicate found", "event_id", evt.ID)
			return false, nil // No duplicate found
		}
		
		mongoLog.Error("Error checking for duplicate event", 
			"event_id", evt.ID,
			"error", err)
		return false, fmt.Errorf("error checking for duplicate event: %v", err)
	}
	
	mongoLog.Info("Duplicate event found", 
		"event_id", evt.ID,
		"kind", evt.Kind,
		"pubkey", evt.PubKey)
	return true, nil // Duplicate found
}