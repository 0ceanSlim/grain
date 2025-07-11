package mongo

import (
	"context"
	"errors"
	"fmt"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// CheckDuplicateEvent checks if an event with the same ID already exists in the collection.
// Returns (isDuplicate, error) where error indicates a system error, not a duplicate.
func CheckDuplicateEvent(ctx context.Context, evt nostr.Event) (bool, error) {
	// First check if the MongoDB client is healthy
	if !IsClientHealthy(ctx) {
		log.Mongo().Warn("MongoDB client is not healthy during duplicate check",
			"event_id", evt.ID,
			"kind", evt.Kind)
		// During restart/reconnection, we allow the event through to avoid blocking
		// This is a reasonable trade-off since duplicates will be caught by unique indexes
		return false, nil
	}

	collection := GetCollection(evt.Kind)
	if collection == nil {
		log.Mongo().Error("Failed to get collection for duplicate check",
			"event_id", evt.ID,
			"kind", evt.Kind)
		// If we can't get collection, allow event through (unique index will catch duplicates)
		return false, nil
	}

	filter := bson.M{"id": evt.ID}

	log.Mongo().Debug("Checking for duplicate event",
		"event_id", evt.ID,
		"kind", evt.Kind,
		"collection", fmt.Sprintf("event-kind%d", evt.Kind))

	var existingEvent nostr.Event
	err := collection.FindOne(ctx, filter).Decode(&existingEvent)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Mongo().Debug("No duplicate found", "event_id", evt.ID)
			return false, nil // No duplicate found
		}

		// Check if this is a connection-related error
		if isConnectionError(err) {
			log.Mongo().Warn("Connection error during duplicate check, allowing event through",
				"event_id", evt.ID,
				"error", err)
			// During connection issues, allow event through - unique index will prevent actual duplicates
			return false, nil
		}

		log.Mongo().Error("Error checking for duplicate event",
			"event_id", evt.ID,
			"error", err)
		return false, fmt.Errorf("error checking for duplicate event: %v", err)
	}

	log.Mongo().Info("Duplicate event found",
		"event_id", evt.ID,
		"kind", evt.Kind,
		"pubkey", evt.PubKey)
	return true, nil // Duplicate found
}
