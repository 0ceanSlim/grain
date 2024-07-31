package kinds

import (
	"context"
	"grain/server/handlers/response"
	relay "grain/server/types"

	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/net/websocket"
)

func HandleUnknownKind(ctx context.Context, evt relay.Event, collection *mongo.Collection, ws *websocket.Conn) error {
	// Respond with an OK message indicating the event is not accepted
	response.SendOK(ws, evt.ID, false, "invalid: kind is outside the ranges defined in NIP01")

	// Return nil as there's no error in the process, just that the event is not accepted
	return nil
}
