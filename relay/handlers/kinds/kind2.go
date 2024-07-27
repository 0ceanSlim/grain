package kinds

import (
	"context"
	"grain/relay/handlers/response"
	relay "grain/relay/types"

	"golang.org/x/net/websocket"
)

func HandleKind2(ctx context.Context, evt relay.Event, ws *websocket.Conn) error {
	// Send a NOTICE message to inform the client about the deprecation
	response.SendNotice(ws, evt.PubKey, "kind 2 is deprecated, event not accepted to the relay, please use kind 10002 as defined in NIP-65")

	// Send an OK message to indicate the event was not accepted
	response.SendOK(ws, evt.ID, false, "invalid: kind 2 is deprecated, use kind 10002 (NIP65)")

	return nil
}
