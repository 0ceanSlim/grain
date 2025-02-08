package kinds

import (
	"context"
	"fmt"
	"grain/server/handlers/response"
	relay "grain/server/types"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/net/websocket"
)

func HandleDeleteKind(ctx context.Context, evt relay.Event, dbClient *mongo.Client, dbName string, ws *websocket.Conn) error {
	for _, tag := range evt.Tags {
		if len(tag) < 2 {
			continue
		}
		if tag[0] == "e" {
			eventID := tag[1]
			if err := deleteEventByID(ctx, dbName, eventID, evt.PubKey, dbClient); err != nil {
				response.SendOK(ws, evt.ID, false, fmt.Sprintf("error: %v", err))
				return fmt.Errorf("error deleting event with ID %s: %v", eventID, err)
			}
		} else if tag[0] == "a" {
			parts := splitTagA(tag[1])
			if len(parts) == 3 {
				kind := parts[0]
				pubKey := parts[1]
				dID := parts[2]

				if err := deletePreviousKind5Events(ctx, dbName, kind, pubKey, dID, dbClient); err != nil {
					response.SendOK(ws, evt.ID, false, fmt.Sprintf("error: %v", err))
					return fmt.Errorf("error deleting previous kind 5 events: %v", err)
				}

				if err := deleteEventByKindPubKeyDID(ctx, dbName, kind, pubKey, dID, evt.CreatedAt, dbClient); err != nil {
					response.SendOK(ws, evt.ID, false, fmt.Sprintf("error: %v", err))
					return fmt.Errorf("error deleting events with kind %s, pubkey %s, and dID %s: %v", kind, pubKey, dID, err)
				}
			}
		}
	}

	if err := storeEvent(ctx, dbName, evt, dbClient); err != nil {
		response.SendOK(ws, evt.ID, false, fmt.Sprintf("error: %v", err))
		return fmt.Errorf("error storing deletion event: %v", err)
	}

	response.SendOK(ws, evt.ID, true, "")
	return nil
}

func deletePreviousKind5Events(ctx context.Context, dbName string, kind string, pubKey string, dID string, dbClient *mongo.Client) error {
	collection := dbClient.Database(dbName).Collection("event-kind5") // ✅ Use dbName
	filter := bson.M{
		"tags": bson.M{
			"$elemMatch": bson.M{
				"0": "a",
				"1": fmt.Sprintf("%s:%s:%s", kind, pubKey, dID),
			},
		},
	}

	_, err := collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("error deleting previous kind 5 events from collection event-kind5: %v", err)
	}

	fmt.Printf("Deleted previous kind 5 events for kind %s, pubkey %s, and dID %s\n", kind, pubKey, dID)
	return nil
}

func deleteEventByID(ctx context.Context, dbName string, eventID string, pubKey string, dbClient *mongo.Client) error {
	collections, err := dbClient.Database(dbName).ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("error listing collections: %v", err)
	}

	for _, collectionName := range collections {
		filter := bson.M{"id": eventID, "pubkey": pubKey}
		result, err := dbClient.Database(dbName).Collection(collectionName).DeleteOne(ctx, filter)
		if err != nil {
			return fmt.Errorf("error deleting event from collection %s: %v", collectionName, err)
		}
		if result.DeletedCount > 0 {
			fmt.Printf("Deleted event %s from collection %s\n", eventID, collectionName)
			return nil
		}
	}

	return nil
}

func splitTagA(tagA string) []string {
	return strings.Split(tagA, ":")
}

func deleteEventByKindPubKeyDID(ctx context.Context, dbName string, kind string, pubKey string, dID string, createdAt int64, dbClient *mongo.Client) error {
	filter := bson.M{"kind": kind, "pubkey": pubKey, "tags.d": dID, "createdat": bson.M{"$lte": createdAt}}
	collections, err := dbClient.Database(dbName).ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("error listing collections: %v", err)
	}

	for _, collectionName := range collections {
		_, err := dbClient.Database(dbName).Collection(collectionName).DeleteMany(ctx, filter)
		if err != nil {
			return fmt.Errorf("error deleting events from collection %s: %v", collectionName, err)
		}
		fmt.Printf("Deleted events with kind %s, pubkey %s, and dID %s from collection %s\n", kind, pubKey, dID, collectionName)
	}

	return nil
}

func storeEvent(ctx context.Context, dbName string, evt relay.Event, dbClient *mongo.Client) error {
	_, err := dbClient.Database(dbName).Collection("event-kind5").InsertOne(ctx, evt)
	if err != nil {
		return fmt.Errorf("error inserting deletion event: %v", err)
	}
	fmt.Printf("Stored deletion event %s\n", evt.ID)
	return nil
}
