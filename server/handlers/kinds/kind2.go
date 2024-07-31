package kinds

import (
	"context"
	"grain/server/handlers/response"
	relay "grain/server/types"

	"golang.org/x/net/websocket"
)

func HandleKind2(ctx context.Context, evt relay.Event, ws *websocket.Conn) error {

	// Send an OK message to indicate the event was not accepted
	response.SendOK(ws, evt.ID, false, "invalid: kind 2 is deprecated, use kind 10002 (NIP65)")

	return nil
}
