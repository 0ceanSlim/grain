package kinds

import (
	"context"
	"fmt"
	"grain/server/handlers/response"
	relay "grain/server/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/net/websocket"
)

// HandleAddressableKind handles parameterized replaceable events based on NIP-01 rules
func HandleAddressableKind(ctx context.Context, evt relay.Event, collection *mongo.Collection, ws *websocket.Conn) error {
	// Step 1: Extract the dTag from the event's tags
	var dTag string
	for _, tag := range evt.Tags {
		if len(tag) > 0 && tag[0] == "d" {
			dTag = tag[1]
			break
		}
	}

	// Step 2: Create a filter to find the existing event based on pubkey, kind, and dTag
	filter := bson.M{"pubkey": evt.PubKey, "kind": evt.Kind, "tags": bson.M{"$elemMatch": bson.M{"0": "d", "1": dTag}}}

	// Step 3: Check if an existing event is found
	var existingEvent relay.Event
	err := collection.FindOne(ctx, filter).Decode(&existingEvent)
	if err != nil && err != mongo.ErrNoDocuments {
		return fmt.Errorf("error finding existing event: %v", err)
	}

	// Step 4: If an existing event is found, compare created_at and id to decide if it should be replaced
	if err != mongo.ErrNoDocuments {
		if existingEvent.CreatedAt > evt.CreatedAt || (existingEvent.CreatedAt == evt.CreatedAt && existingEvent.ID < evt.ID) {
			response.SendOK(ws, evt.ID, false, "blocked: relay already has a newer event for this pubkey and dTag")
			return nil
		}

		// Step 5: Delete the older event before inserting the new one
		_, err := collection.DeleteOne(ctx, filter)
		if err != nil {
			return fmt.Errorf("error deleting the older event: %v", err)
		}
		fmt.Printf("Deleted older event with ID: %s\n", existingEvent.ID)
	}

	// Step 6: Insert the new event (without upsert since we already deleted the old one)
	_, err = collection.InsertOne(ctx, evt)
	if err != nil {
		response.SendOK(ws, evt.ID, false, "error: could not insert the new event into the database")
		return fmt.Errorf("error inserting event kind %d into MongoDB: %v", evt.Kind, err)
	}

	fmt.Printf("Inserted event kind %d into MongoDB: %s\n", evt.Kind, evt.ID)
	response.SendOK(ws, evt.ID, true, "")
	return nil
}
