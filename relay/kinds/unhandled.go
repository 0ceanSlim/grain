package kinds

import (
	"context"
	"fmt"
	relay "grain/relay/types"

	"go.mongodb.org/mongo-driver/mongo"
)

func HandleUnknownKind(ctx context.Context, evt relay.Event, collection *mongo.Collection) error {
	_, err := collection.InsertOne(ctx, evt)
	if err != nil {
		return fmt.Errorf("Error inserting unknown event into MongoDB: %v", err)
	}

	fmt.Println("Inserted unknown event into MongoDB:", evt.ID)
	return nil
}
