package kinds

import (
	"context"
	"fmt"

	server "grain/server/types"

	"go.mongodb.org/mongo-driver/mongo"
)

func HandleKind1(ctx context.Context, evt server.Event, collection *mongo.Collection) error {
	// Insert event into MongoDB
	_, err := collection.InsertOne(ctx, evt)
	if err != nil {
		return fmt.Errorf("Error inserting event into MongoDB: %v", err)
	}

	fmt.Println("Inserted event kind 1 into MongoDB:", evt.ID)
	return nil
}
