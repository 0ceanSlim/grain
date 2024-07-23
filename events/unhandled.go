package events

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
)

func HandleUnknownEvent(ctx context.Context, evt Event, collection *mongo.Collection) error {
	_, err := collection.InsertOne(ctx, evt)
	if err != nil {
		return fmt.Errorf("Error inserting unknown event into MongoDB: %v", err)
	}

	fmt.Println("Inserted unknown event into MongoDB:", evt.ID)
	return nil
}
