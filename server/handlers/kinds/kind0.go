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

func HandleKind0(ctx context.Context, evt relay.Event, collection *mongo.Collection, ws *websocket.Conn) error {
	filter := bson.M{"pubkey": evt.PubKey}
	var existingEvent relay.Event
	err := collection.FindOne(ctx, filter).Decode(&existingEvent)
	if err != nil && err != mongo.ErrNoDocuments {
		return fmt.Errorf("error finding existing event: %v", err)
	}

	if err != mongo.ErrNoDocuments {
		if existingEvent.CreatedAt >= evt.CreatedAt {
			response.SendOK(ws, evt.ID, false, "blocked: a newer kind 0 event already exists for this pubkey")
			return nil
		}
	}

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

	opts := options.Update().SetUpsert(true)
	_, err = collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		response.SendOK(ws, evt.ID, false, "error: could not connect to the database")
		return fmt.Errorf("error updating/inserting event kind 0 into MongoDB: %v", err)
	}
	response.SendOK(ws, evt.ID, true, "")
	fmt.Println("Upserted event kind 0 into MongoDB:", evt.ID)
	return nil
}
