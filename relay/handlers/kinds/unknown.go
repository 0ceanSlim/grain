package kinds

import (
	"context"
	"encoding/json"
	relay "grain/relay/types"

	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/net/websocket"
)

func HandleUnknownKind(ctx context.Context, evt relay.Event, collection *mongo.Collection, ws *websocket.Conn) error {
	// Respond with an OK message indicating the event is not accepted
	sendOK(ws, evt.ID, false, "kind is unknown and not accepted")

	// Return nil as there's no error in the process, just that the event is not accepted
	return nil
}
func sendOK(ws *websocket.Conn, eventID string, status bool, message string) {
	response := []interface{}{"OK", eventID, status, message}
	responseBytes, _ := json.Marshal(response)
	websocket.Message.Send(ws, string(responseBytes))
}