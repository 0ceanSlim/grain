package eventStore

import (
	"context"
	"fmt"

	"github.com/0ceanslim/grain/server/handlers/response"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"go.mongodb.org/mongo-driver/mongo"
)

// Regular stores regular events in the database
func Regular(ctx context.Context, evt nostr.Event, collection *mongo.Collection, client nostr.ClientInterface) error {
	result, err := collection.InsertOne(ctx, evt)
	if err != nil {
		log.EventStore().Error("Failed to insert regular event", 
			"event_id", evt.ID, 
			"kind", evt.Kind, 
			"pubkey", evt.PubKey, 
			"error", err)
		response.SendOK(client, evt.ID, false, "error: could not connect to the database")
		return fmt.Errorf("error inserting event kind %d into MongoDB: %v", evt.Kind, err)
	}

	log.EventStore().Info("Inserted regular event", 
		"event_id", evt.ID, 
		"kind", evt.Kind, 
		"pubkey", evt.PubKey, 
		"inserted_id", result.InsertedID)
	response.SendOK(client, evt.ID, true, "")
	return nil
}