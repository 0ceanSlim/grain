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
	_, err := collection.InsertOne(ctx, evt)
	if err != nil {
		response.SendOK(client, evt.ID, false, "error: could not connect to the database")
		return fmt.Errorf("error inserting event kind %d into MongoDB: %v", evt.Kind, err)
	}

	fmt.Printf("Inserted event kind %d into MongoDB: %s\n", evt.Kind, evt.ID)
	response.SendOK(client, evt.ID, true, "")
	return nil
}
