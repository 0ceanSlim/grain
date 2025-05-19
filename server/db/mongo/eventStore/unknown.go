package eventStore

import (
	"context"

	"github.com/0ceanslim/grain/server/handlers/response"
	relay "github.com/0ceanslim/grain/server/types"

	"go.mongodb.org/mongo-driver/mongo"
)

// Unknown rejects events with unknown kinds
func Unknown(ctx context.Context, evt relay.Event, collection *mongo.Collection, client relay.ClientInterface) error {
	esLog().Warn("Rejecting unknown event kind", 
		"event_id", evt.ID, 
		"kind", evt.Kind, 
		"pubkey", evt.PubKey)
	
	// Respond with an OK message indicating the event is not accepted
	response.SendOK(client, evt.ID, false, "invalid: kind is outside the ranges defined in NIP01")

	// Return nil as there's no error in the process, just that the event is not accepted
	return nil
}