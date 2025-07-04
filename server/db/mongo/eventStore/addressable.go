package eventStore

import (
	"context"
	"fmt"

	"github.com/0ceanslim/grain/server/handlers/response"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Addressable handles parameterized replaceable events based on NIP-01 rules
func Addressable(ctx context.Context, evt nostr.Event, collection *mongo.Collection, client nostr.ClientInterface) error {
	// Step 1: Extract the dTag from the event's tags
	var dTag string
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "d" {
			dTag = tag[1]
			break
		}
	}

	if dTag == "" {
		log.EventStore().Warn("No d tag found in addressable event",
			"event_id", evt.ID,
			"kind", evt.Kind,
			"pubkey", evt.PubKey)
		return fmt.Errorf("no d tag is present in addressable event")
	}

	// Step 2: Create a filter to find the existing event based on pubkey, kind, and dTag
	filter := bson.M{"pubkey": evt.PubKey, "kind": evt.Kind, "tags": bson.M{"$elemMatch": bson.M{"0": "d", "1": dTag}}}

	// Step 3: Check if an existing event is found
	var existingEvent nostr.Event
	err := collection.FindOne(ctx, filter).Decode(&existingEvent)
	if err != nil && err != mongo.ErrNoDocuments {
		log.EventStore().Error("Failed to find existing event",
			"event_id", evt.ID,
			"kind", evt.Kind,
			"pubkey", evt.PubKey,
			"d_tag", dTag,
			"error", err)
		return fmt.Errorf("error finding existing event: %v", err)
	}

	// Step 4: If an existing event is found, compare created_at and id to decide if it should be replaced
	if err != mongo.ErrNoDocuments {
		log.EventStore().Debug("Found existing addressable event",
			"existing_id", existingEvent.ID,
			"new_id", evt.ID,
			"existing_created_at", existingEvent.CreatedAt,
			"new_created_at", evt.CreatedAt)

		if existingEvent.CreatedAt > evt.CreatedAt || (existingEvent.CreatedAt == evt.CreatedAt && existingEvent.ID < evt.ID) {
			log.EventStore().Info("Rejecting event - newer version exists",
				"event_id", evt.ID,
				"existing_id", existingEvent.ID,
				"kind", evt.Kind,
				"pubkey", evt.PubKey,
				"d_tag", dTag)
			response.SendOK(client, evt.ID, false, "blocked: relay already has a newer event for this pubkey and dTag")
			return nil
		}

		// Step 5: Delete the older event before inserting the new one
		result, err := collection.DeleteOne(ctx, filter)
		if err != nil {
			log.EventStore().Error("Failed to delete older event",
				"existing_id", existingEvent.ID,
				"new_id", evt.ID,
				"kind", evt.Kind,
				"pubkey", evt.PubKey,
				"error", err)
			return fmt.Errorf("error deleting the older event: %v", err)
		}
		log.EventStore().Info("Deleted older event",
			"existing_id", existingEvent.ID,
			"new_id", evt.ID,
			"kind", evt.Kind,
			"deleted_count", result.DeletedCount)
	}

	// Step 6: Insert the new event (without upsert since we already deleted the old one)
	result, err := collection.InsertOne(ctx, evt)
	if err != nil {
		log.EventStore().Error("Failed to insert addressable event",
			"event_id", evt.ID,
			"kind", evt.Kind,
			"pubkey", evt.PubKey,
			"error", err)
		response.SendOK(client, evt.ID, false, "error: could not insert the new event into the database")
		return fmt.Errorf("error inserting event kind %d into MongoDB: %v", evt.Kind, err)
	}

	log.EventStore().Info("Inserted addressable event",
		"event_id", evt.ID,
		"kind", evt.Kind,
		"pubkey", evt.PubKey,
		"d_tag", dTag,
		"inserted_id", result.InsertedID)
	response.SendOK(client, evt.ID, true, "")
	return nil
}
