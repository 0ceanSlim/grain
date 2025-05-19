package eventStore

import (
	"context"

	"github.com/0ceanslim/grain/server/handlers/response"
	relay "github.com/0ceanslim/grain/server/types"
)

// Deprecated rejects kind 2 events since they are deprecated
func Deprecated(ctx context.Context, evt relay.Event, client relay.ClientInterface) error {
	esLog().Info("Rejecting deprecated event kind 2", 
		"event_id", evt.ID, 
		"pubkey", evt.PubKey)
	
	// Send an OK message to indicate the event was not accepted
	response.SendOK(client, evt.ID, false, "invalid: kind 2 is deprecated, use kind 10002 (NIP65)")
	return nil
}