package eventStore

import (
	"context"
	"fmt"

	"github.com/0ceanslim/grain/server/handlers/response"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Replaceable manages replaceable events by updating or inserting them
func Replaceable(ctx context.Context, evt nostr.Event, collection *mongo.Collection, client nostr.ClientInterface) error {
	filter := bson.M{"pubkey": evt.PubKey, "kind": evt.Kind}

	// Check if an existing event is found
	var existingEvent nostr.Event
	err := collection.FindOne(ctx, filter).Decode(&existingEvent)
	if err != nil && err != mongo.ErrNoDocuments {
		log.EventStore().Error("Failed to find existing replaceable event",
			"event_id", evt.ID,
			"kind", evt.Kind,
			"pubkey", evt.PubKey,
			"error", err)
		return fmt.Errorf("error finding existing event: %v", err)
	}

	// If an existing event is found, compare timestamps
	if err != mongo.ErrNoDocuments {
		log.EventStore().Debug("Found existing replaceable event",
			"existing_id", existingEvent.ID,
			"new_id", evt.ID,
			"existing_created_at", existingEvent.CreatedAt,
			"new_created_at", evt.CreatedAt)

		if existingEvent.CreatedAt > evt.CreatedAt || (existingEvent.CreatedAt == evt.CreatedAt && existingEvent.ID < evt.ID) {
			log.EventStore().Info("Rejecting event - newer version exists",
				"event_id", evt.ID,
				"existing_id", existingEvent.ID,
				"kind", evt.Kind,
				"pubkey", evt.PubKey)
			response.SendOK(client, evt.ID, false, "blocked: relay already has a newer event of the same kind with this pubkey")
			return nil
		}
	}

	// Upsert the event
	opts := options.Update().SetUpsert(true)
	update := bson.M{
		"$set": evt,
	}

	result, err := collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		log.EventStore().Error("Failed to upsert replaceable event",
			"event_id", evt.ID,
			"kind", evt.Kind,
			"pubkey", evt.PubKey,
			"error", err)
		response.SendOK(client, evt.ID, false, "error: could not connect to the database")
		return fmt.Errorf("error updating/inserting event kind %d into MongoDB: %v", evt.Kind, err)
	}

	// log.EventStore() appropriate message based on whether it was an update or insert
	if result.MatchedCount > 0 {
		log.EventStore().Info("Updated replaceable event",
			"event_id", evt.ID,
			"kind", evt.Kind,
			"pubkey", evt.PubKey,
			"matched_count", result.MatchedCount,
			"modified_count", result.ModifiedCount)
	} else {
		log.EventStore().Info("Inserted replaceable event",
			"event_id", evt.ID,
			"kind", evt.Kind,
			"pubkey", evt.PubKey,
			"upserted_id", result.UpsertedID)
	}

	response.SendOK(client, evt.ID, true, "")
	return nil
}
