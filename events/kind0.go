package events

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/mongo"
)

func HandleEventKind0(ctx context.Context, evt Event, collection *mongo.Collection) error {
	// Perform specific validation for event kind 1
	if !isValidEventKind0(evt) {
		return fmt.Errorf("validation failed for event kind 0: %s", evt.ID)
	}

	// Insert event into MongoDB
	_, err := collection.InsertOne(ctx, evt)
	if err != nil {
		fmt.Println("Error inserting event into MongoDB:", err)
		return err
	}

	fmt.Println("Inserted event kind 0 into MongoDB:", evt.ID)
	return nil
}

func isValidEventKind0(evt Event) bool {
	// Placeholder for actual validation logic
	if evt.Content == "" {
		return false
	}
	return true
}
