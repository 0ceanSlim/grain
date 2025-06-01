package eventStore

import (
	"context"
	"fmt"
	"strings"

	"github.com/0ceanslim/grain/server/handlers/response"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// Delete processes kind 5 delete events and removes matching events from the database
func Delete(ctx context.Context, evt nostr.Event, dbClient *mongo.Client, dbName string, client nostr.ClientInterface) error {
	log.EventStore().Info("Processing deletion event", 
		"event_id", evt.ID, 
		"pubkey", evt.PubKey,
		"tag_count", len(evt.Tags))

	for _, tag := range evt.Tags {
		if len(tag) < 2 {
			continue
		}

		if tag[0] == "e" {
			eventID := tag[1]
			log.EventStore().Debug("Processing e tag deletion", 
				"target_event_id", eventID, 
				"event_id", evt.ID)

			if err := deleteSpecificEvent(ctx, dbName, eventID, evt.PubKey, dbClient); err != nil {
				log.EventStore().Error("Failed to delete event by ID", 
					"target_event_id", eventID, 
					"event_id", evt.ID, 
					"error", err)
				response.SendOK(client, evt.ID, false, fmt.Sprintf("error: %v", err))
				return fmt.Errorf("error deleting event with ID %s: %v", eventID, err)
			}
		} else if tag[0] == "a" {
			parts := parseAddressableEventReference(tag[1])
			if len(parts) == 3 {
				kind := parts[0]
				pubKey := parts[1]
				dID := parts[2]

				log.EventStore().Debug("Processing a tag deletion", 
					"kind", kind, 
					"pubkey", pubKey, 
					"d_tag", dID, 
					"event_id", evt.ID)

				if err := cleanupPreviousDeletionRequests(ctx, dbName, kind, pubKey, dID, dbClient); err != nil {
					log.EventStore().Error("Failed to delete previous kind 5 events", 
						"kind", kind, 
						"pubkey", pubKey, 
						"d_tag", dID, 
						"error", err)
					response.SendOK(client, evt.ID, false, fmt.Sprintf("error: %v", err))
					return fmt.Errorf("error deleting previous kind 5 events: %v", err)
				}

				if err := deleteAddressableEvents(ctx, dbName, kind, pubKey, dID, evt.CreatedAt, dbClient); err != nil {
					log.EventStore().Error("Failed to delete events by kind, pubkey, and dID", 
						"kind", kind, 
						"pubkey", pubKey, 
						"d_tag", dID, 
						"error", err)
					response.SendOK(client, evt.ID, false, fmt.Sprintf("error: %v", err))
					return fmt.Errorf("error deleting events with kind %s, pubkey %s, and dID %s: %v", kind, pubKey, dID, err)
				}
			}
		}
	}

	if err := storeDeletionEvent(ctx, dbName, evt, dbClient); err != nil {
		log.EventStore().Error("Failed to store deletion event", 
			"event_id", evt.ID, 
			"error", err)
		response.SendOK(client, evt.ID, false, fmt.Sprintf("error: %v", err))
		return fmt.Errorf("error storing deletion event: %v", err)
	}

	log.EventStore().Info("Deletion event processed successfully", "event_id", evt.ID)
	response.SendOK(client, evt.ID, true, "")
	return nil
}

func cleanupPreviousDeletionRequests(ctx context.Context, dbName string, kind string, pubKey string, dID string, dbClient *mongo.Client) error {
	collection := dbClient.Database(dbName).Collection("event-kind5")
	filter := bson.M{
		"tags": bson.M{
			"$elemMatch": bson.M{
				"0": "a",
				"1": fmt.Sprintf("%s:%s:%s", kind, pubKey, dID),
			},
		},
	}

	result, err := collection.DeleteMany(ctx, filter)
	if err != nil {
		log.EventStore().Error("Failed to delete previous kind 5 events", 
			"kind", kind, 
			"pubkey", pubKey, 
			"d_tag", dID, 
			"error", err)
		return fmt.Errorf("error deleting previous kind 5 events from collection event-kind5: %v", err)
	}

	log.EventStore().Info("Deleted previous kind 5 events", 
		"kind", kind, 
		"pubkey", pubKey, 
		"d_tag", dID, 
		"deleted_count", result.DeletedCount)
	return nil
}

func deleteSpecificEvent(ctx context.Context, dbName string, eventID string, pubKey string, dbClient *mongo.Client) error {
	collections, err := dbClient.Database(dbName).ListCollectionNames(ctx, bson.M{})
	if err != nil {
		log.EventStore().Error("Failed to list collections", 
			"database", dbName, 
			"error", err)
		return fmt.Errorf("error listing collections: %v", err)
	}

	log.EventStore().Debug("Searching for event across collections", 
		"event_id", eventID, 
		"pubkey", pubKey, 
		"collection_count", len(collections))

	for _, collectionName := range collections {
		filter := bson.M{"id": eventID, "pubkey": pubKey}
		result, err := dbClient.Database(dbName).Collection(collectionName).DeleteOne(ctx, filter)
		if err != nil {
			log.EventStore().Error("Failed to delete event from collection", 
				"collection", collectionName, 
				"event_id", eventID, 
				"error", err)
			return fmt.Errorf("error deleting event from collection %s: %v", collectionName, err)
		}
		if result.DeletedCount > 0 {
			log.EventStore().Info("Successfully deleted event", 
				"event_id", eventID, 
				"collection", collectionName)
			return nil
		}
	}

	log.EventStore().Debug("No matching event found to delete", 
		"event_id", eventID, 
		"pubkey", pubKey)
	return nil
}

func deleteAddressableEvents(ctx context.Context, dbName string, kind string, pubKey string, dID string, createdAt int64, dbClient *mongo.Client) error {
	filter := bson.M{"kind": kind, "pubkey": pubKey, "tags.d": dID, "created_at": bson.M{"$lte": createdAt}}
	collections, err := dbClient.Database(dbName).ListCollectionNames(ctx, bson.M{})
	if err != nil {
		log.EventStore().Error("Failed to list collections", 
			"database", dbName, 
			"error", err)
		return fmt.Errorf("error listing collections: %v", err)
	}

	for _, collectionName := range collections {
		result, err := dbClient.Database(dbName).Collection(collectionName).DeleteMany(ctx, filter)
		if err != nil {
			log.EventStore().Error("Failed to delete events from collection", 
				"collection", collectionName, 
				"kind", kind, 
				"pubkey", pubKey, 
				"d_tag", dID, 
				"error", err)
			return fmt.Errorf("error deleting events from collection %s: %v", collectionName, err)
		}
		
		if result.DeletedCount > 0 {
			log.EventStore().Info("Deleted events by kind, pubkey, and dID", 
				"collection", collectionName, 
				"kind", kind, 
				"pubkey", pubKey, 
				"d_tag", dID, 
				"deleted_count", result.DeletedCount)
		}
	}

	return nil
}

func storeDeletionEvent(ctx context.Context, dbName string, evt nostr.Event, dbClient *mongo.Client) error {
	collection := dbClient.Database(dbName).Collection("event-kind5")
	result, err := collection.InsertOne(ctx, evt)
	if err != nil {
		log.EventStore().Error("Failed to insert deletion event", 
			"event_id", evt.ID, 
			"error", err)
		return fmt.Errorf("error inserting deletion event: %v", err)
	}
	
	log.EventStore().Info("Stored deletion event", 
		"event_id", evt.ID, 
		"inserted_id", result.InsertedID)
	return nil
}

func parseAddressableEventReference(tagA string) []string {
	return strings.Split(tagA, ":")
}