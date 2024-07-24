package kinds

import (
	"context"
	"fmt"
	relay "grain/relay/types"

	"go.mongodb.org/mongo-driver/mongo"
)

func HandleRegularKind(ctx context.Context, evt relay.Event, collection *mongo.Collection) error {
	_, err := collection.InsertOne(ctx, evt)
	if err != nil {
		return fmt.Errorf("error inserting event kind %d into MongoDB: %v", evt.Kind, err)
	}

	fmt.Printf("Inserted event kind %d into MongoDB: %s\n", evt.Kind, evt.ID)
	return nil
}