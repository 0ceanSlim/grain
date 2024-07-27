package kinds

import (
	"context"
	"fmt"
	"grain/relay/handlers/response"
	relay "grain/relay/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/websocket"
)

func HandleReplaceableKind(ctx context.Context, evt relay.Event, collection *mongo.Collection, ws *websocket.Conn) error {
	filter := bson.M{"pubkey": evt.PubKey, "kind": evt.Kind}
	var existingEvent relay.Event
	err := collection.FindOne(ctx, filter).Decode(&existingEvent)
	if err != nil && err != mongo.ErrNoDocuments {
		return fmt.Errorf("error finding existing event: %v", err)
	}

	if err != mongo.ErrNoDocuments {
		if existingEvent.CreatedAt > evt.CreatedAt || (existingEvent.CreatedAt == evt.CreatedAt && existingEvent.ID < evt.ID) {
			response.SendOK(ws, evt.ID, false, "blocked: relay already has a newer event of the same kind with this pubkey")
			return nil
		}
	}

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
