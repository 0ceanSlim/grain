package kinds

import (
	"context"
	"fmt"
	"grain/server/handlers/response"
	relay "grain/server/types"

	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/net/websocket"
)

func HandleRegularKind(ctx context.Context, evt relay.Event, collection *mongo.Collection, ws *websocket.Conn) error {
	_, err := collection.InsertOne(ctx, evt)
	if err != nil {
		response.SendOK(ws, evt.ID, false, "error: could not connect to the database")
		return fmt.Errorf("error inserting event kind %d into MongoDB: %v", evt.Kind, err)
	}

	fmt.Printf("Inserted event kind %d into MongoDB: %s\n", evt.Kind, evt.ID)
	response.SendOK(ws, evt.ID, true, "")
	return nil
}
