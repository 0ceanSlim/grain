package kinds

import (
	"context"
	"fmt"
	"grain/server/handlers/response"
	relay "grain/server/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/websocket"
)

// HandleParameterizedReplaceableKind handles parameterized replaceable events based on NIP-01 rules
func HandleParameterizedReplaceableKind(ctx context.Context, evt relay.Event, collection *mongo.Collection, ws *websocket.Conn) error {
	// Step 1: Find dTag from the event's tags
	var dTag string
	for _, tag := range evt.Tags {
		if len(tag) > 0 && tag[0] == "d" {
			dTag = tag[1]
			break
		}
	}

	// Step 2: Create filter to find the existing event based on pubkey, kind, and dTag
	filter := bson.M{"pubkey": evt.PubKey, "kind": evt.Kind, "tags.d": dTag}

	// Step 3: Find the existing event from the database
	var existingEvent relay.Event
	err := collection.FindOne(ctx, filter).Decode(&existingEvent)
	if err != nil && err != mongo.ErrNoDocuments {
		return fmt.Errorf("error finding existing event: %v", err)
	}

	// Step 4: Handle event replacement logic (NIP-01 rules)
	// If we found an existing event and the new event is older, reject the update
	if err != mongo.ErrNoDocuments {
		if existingEvent.CreatedAt > evt.CreatedAt || (existingEvent.CreatedAt == evt.CreatedAt && existingEvent.ID < evt.ID) {
			response.SendOK(ws, evt.ID, false, "blocked: relay already has a newer event for this pubkey and dTag")
			return nil
		}

		// Step 5: Delete the older event if the new event is valid
		_, err := collection.DeleteOne(ctx, bson.M{"_id": existingEvent.ID})
		if err != nil {
			return fmt.Errorf("error deleting the older event: %v", err)
		}
		fmt.Printf("Deleted older event with ID: %s\n", existingEvent.ID)
	}

	// Step 6: Upsert the new event (insert if not existing, or update if newer)
	opts := options.Update().SetUpsert(true)
	update := bson.M{
		"$set": evt,
	}
	_, err = collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		response.SendOK(ws, evt.ID, false, "error: could not connect to the database")
		return fmt.Errorf("error updating/inserting event kind %d into MongoDB: %v", evt.Kind, err)
	}

	fmt.Printf("Upserted event kind %d into MongoDB: %s\n", evt.Kind, evt.ID)
	response.SendOK(ws, evt.ID, true, "")
	return nil
}
