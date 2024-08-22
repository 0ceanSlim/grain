package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"grain/config"
	"grain/server/db"
	"grain/server/handlers/response"
	relay "grain/server/types"
	"grain/server/utils"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/websocket"
)

var subscriptions = make(map[string]relay.Subscription)
var mu sync.Mutex // Protect concurrent access to subscriptions map

func HandleReq(ws *websocket.Conn, message []interface{}, subscriptions map[string][]relay.Filter) {
	utils.LimitedGoRoutine(func() {
		if len(message) < 3 {
			fmt.Println("Invalid REQ message format")
			response.SendClosed(ws, "", "invalid: invalid REQ message format")
			return
		}

		subID, ok := message[1].(string)
		if !ok {
			fmt.Println("Invalid subscription ID format")
			response.SendClosed(ws, "", "invalid: invalid subscription ID format")
			return
		}

		mu.Lock()
		defer mu.Unlock()

		// Check the current number of subscriptions for the client
		if len(subscriptions) >= config.GetConfig().Server.MaxSubscriptionsPerClient {
			// Find and remove the oldest subscription (FIFO)
			var oldestSubID string
			for id := range subscriptions {
				oldestSubID = id
				break
			}
			delete(subscriptions, oldestSubID)
			fmt.Println("Dropped oldest subscription:", oldestSubID)
		}

		// Prepare filters based on the incoming message
		filters := make([]relay.Filter, len(message)-2)
		for i, filter := range message[2:] {
			filterData, ok := filter.(map[string]interface{})
			if !ok {
				fmt.Println("Invalid filter format")
				response.SendClosed(ws, subID, "invalid: invalid filter format")
				return
			}

			var f relay.Filter
			f.IDs = utils.ToStringArray(filterData["ids"])
			f.Authors = utils.ToStringArray(filterData["authors"])
			f.Kinds = utils.ToIntArray(filterData["kinds"])
			f.Tags = utils.ToTagsMap(filterData["tags"])
			f.Since = utils.ToTime(filterData["since"])
			f.Until = utils.ToTime(filterData["until"])
			f.Limit = utils.ToInt(filterData["limit"])

			filters[i] = f
		}

		// Add the new subscription or update the existing one
		subscriptions[subID] = filters
		fmt.Printf("Subscription updated: %s with %d filters\n", subID, len(filters))

		// Query the database with filters and send back the results
		queriedEvents, err := QueryEvents(filters, db.GetClient(), "grain")
		if err != nil {
			fmt.Println("Error querying events:", err)
			response.SendClosed(ws, subID, "error: could not query events")
			return
		}

		// Log the number of events retrieved
		fmt.Printf("Retrieved %d events for subscription %s\n", len(queriedEvents), subID)
		if len(queriedEvents) == 0 {
			fmt.Printf("No matching events found for subscription %s\n", subID)
		}

		// Send each event back to the client
		for _, evt := range queriedEvents {
			msg := []interface{}{"EVENT", subID, evt}
			msgBytes, _ := json.Marshal(msg)
			err = websocket.Message.Send(ws, string(msgBytes))
			if err != nil {
				fmt.Println("Error sending event:", err)
				response.SendClosed(ws, subID, "error: could not send event")
				return
			}
		}

		// Indicate end of stored events
		eoseMsg := []interface{}{"EOSE", subID}
		eoseBytes, _ := json.Marshal(eoseMsg)
		err = websocket.Message.Send(ws, string(eoseBytes))
		if err != nil {
			fmt.Println("Error sending EOSE:", err)
			response.SendClosed(ws, subID, "error: could not send EOSE")
			return
		}

		fmt.Println("Subscription handling completed, keeping connection open.")
	})
}

// QueryEvents queries events from the MongoDB collection(s) based on filters
func QueryEvents(filters []relay.Filter, client *mongo.Client, databaseName string) ([]relay.Event, error) {
	var results []relay.Event

	for _, filter := range filters {
		filterBson := bson.M{}

		// Construct the BSON query based on the filters
		if len(filter.IDs) > 0 {
			filterBson["id"] = bson.M{"$in": filter.IDs}
		}
		if len(filter.Authors) > 0 {
			filterBson["pubkey"] = bson.M{"$in": filter.Authors}
		}
		if len(filter.Kinds) > 0 {
			filterBson["kind"] = bson.M{"$in": filter.Kinds}
		}
		if filter.Tags != nil {
			for key, values := range filter.Tags {
				if len(values) > 0 {
					filterBson["tags."+key] = bson.M{"$in": values}
				}
			}
		}
		if filter.Since != nil {
			filterBson["created_at"] = bson.M{"$gte": *filter.Since}
		}
		if filter.Until != nil {
			if filterBson["created_at"] == nil {
				filterBson["created_at"] = bson.M{"$lte": *filter.Until}
			} else {
				filterBson["created_at"].(bson.M)["$lte"] = *filter.Until
			}
		}

		opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
		if filter.Limit != nil {
			opts.SetLimit(int64(*filter.Limit))
		}

		// If no specific kinds are specified, query all collections in the database
		if len(filter.Kinds) == 0 {
			collections, err := client.Database(databaseName).ListCollectionNames(context.TODO(), bson.D{})
			if err != nil {
				return nil, fmt.Errorf("error listing collections: %v", err)
			}

			for _, collectionName := range collections {
				fmt.Printf("Querying collection: %s with query: %v\n", collectionName, filterBson)

				collection := client.Database(databaseName).Collection(collectionName)
				cursor, err := collection.Find(context.TODO(), filterBson, opts)
				if err != nil {
					return nil, fmt.Errorf("error querying collection %s: %v", collectionName, err)
				}
				defer cursor.Close(context.TODO())

				for cursor.Next(context.TODO()) {
					var event relay.Event
					if err := cursor.Decode(&event); err != nil {
						return nil, fmt.Errorf("error decoding event from collection %s: %v", collectionName, err)
					}
					results = append(results, event)
				}
				if err := cursor.Err(); err != nil {
					return nil, fmt.Errorf("cursor error in collection %s: %v", collectionName, err)
				}
			}
		} else {
			// Query specific collections based on kinds
			for _, kind := range filter.Kinds {
				collectionName := fmt.Sprintf("event-kind%d", kind)
				fmt.Printf("Querying collection: %s with query: %v\n", collectionName, filterBson)

				collection := client.Database(databaseName).Collection(collectionName)
				cursor, err := collection.Find(context.TODO(), filterBson, opts)
				if err != nil {
					return nil, fmt.Errorf("error querying collection %s: %v", collectionName, err)
				}
				defer cursor.Close(context.TODO())

				for cursor.Next(context.TODO()) {
					var event relay.Event
					if err := cursor.Decode(&event); err != nil {
						return nil, fmt.Errorf("error decoding event from collection %s: %v", collectionName, err)
					}
					results = append(results, event)
				}
				if err := cursor.Err(); err != nil {
					return nil, fmt.Errorf("cursor error in collection %s: %v", collectionName, err)
				}
			}
		}
	}

	return results, nil
}
