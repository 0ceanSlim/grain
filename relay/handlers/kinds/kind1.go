// kinds/kind1.go
package kinds

import (
	"context"
	"fmt"
	"grain/relay/handlers/response"
	relay "grain/relay/types"

	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/net/websocket"
)

func HandleKind1(ctx context.Context, evt relay.Event, collection *mongo.Collection, ws *websocket.Conn) error {
	_, err := collection.InsertOne(ctx, evt)
	if err != nil {
		response.SendOK(ws, evt.ID, false, "error: could not connect to the database")
		return fmt.Errorf("error inserting event into MongoDB: %v", err)
	}

	fmt.Println("Inserted event kind 1 into MongoDB:", evt.ID)
	response.SendOK(ws, evt.ID, true, "")
	return nil
}
