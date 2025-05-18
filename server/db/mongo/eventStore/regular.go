package eventStore

import (
	"context"
	"fmt"

	"github.com/0ceanslim/grain/server/handlers/response"
	relay "github.com/0ceanslim/grain/server/types"

	"go.mongodb.org/mongo-driver/mongo"
)

// Regular stores regular events in the database
func Regular(ctx context.Context, evt relay.Event, collection *mongo.Collection, client relay.ClientInterface) error {
	result, err := collection.InsertOne(ctx, evt)
	if err != nil {
		log.Error("Failed to insert regular event", 
			"event_id", evt.ID, 
			"kind", evt.Kind, 
			"pubkey", evt.PubKey, 
			"error", err)
		response.SendOK(client, evt.ID, false, "error: could not connect to the database")
		return fmt.Errorf("error inserting event kind %d into MongoDB: %v", evt.Kind, err)
	}

	log.Info("Inserted regular event", 
		"event_id", evt.ID, 
		"kind", evt.Kind, 
		"pubkey", evt.PubKey, 
		"inserted_id", result.InsertedID)
	response.SendOK(client, evt.ID, true, "")
	return nil
}