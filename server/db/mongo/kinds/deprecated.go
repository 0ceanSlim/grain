package kinds

import (
	"context"
	"grain/server/handlers/response"
	relay "grain/server/types"
)

// HandleDeprecatedKind rejects kind 2 events since they are deprecated
func HandleDeprecatedKind(ctx context.Context, evt relay.Event, client relay.ClientInterface) error {
	// Send an OK message to indicate the event was not accepted
	response.SendOK(client, evt.ID, false, "invalid: kind 2 is deprecated, use kind 10002 (NIP65)")
	return nil
}
