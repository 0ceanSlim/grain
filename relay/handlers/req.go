package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"grain/relay/db"
	relay "grain/relay/types"
	"grain/relay/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/websocket"
)

var subscriptions = make(map[string]relay.Subscription)

func HandleReq(ws *websocket.Conn, message []interface{}) {
	if len(message) < 3 {
		fmt.Println("Invalid REQ message format")
		SendClosed(ws, "", "invalid: invalid REQ message format")
		return
	}

	subID, ok := message[1].(string)
	if !ok {
		fmt.Println("Invalid subscription ID format")
		SendClosed(ws, "", "invalid: invalid subscription ID format")
		return
	}

	filters := make([]relay.Filter, len(message)-2)
	for i, filter := range message[2:] {
		filterData, ok := filter.(map[string]interface{})
		if !ok {
			fmt.Println("Invalid filter format")
			SendClosed(ws, subID, "invalid: invalid filter format")
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

	// Check if subscription already exists
	if _, exists := subscriptions[subID]; exists {
		SendClosed(ws, subID, "duplicate: subID already opened")
		return
	}

	subscriptions[subID] = relay.Subscription{ID: subID, Filters: filters}
	fmt.Println("Subscription added:", subID)

	// Query the database with filters and send back the results
	queriedEvents, err := QueryEvents(filters, db.GetClient(), "grain")
	if err != nil {
		fmt.Println("Error querying events:", err)
		SendClosed(ws, subID, "error: could not query events")
		return
	}

	for _, evt := range queriedEvents {
		msg := []interface{}{"EVENT", subID, evt}
		msgBytes, _ := json.Marshal(msg)
		err = websocket.Message.Send(ws, string(msgBytes))
		if err != nil {
			fmt.Println("Error sending event:", err)
			SendClosed(ws, subID, "error: could not send event")
			return
		}
	}

	// Indicate end of stored events
	eoseMsg := []interface{}{"EOSE", subID}
	eoseBytes, _ := json.Marshal(eoseMsg)
	err = websocket.Message.Send(ws, string(eoseBytes))
	if err != nil {
		fmt.Println("Error sending EOSE:", err)
		SendClosed(ws, subID, "error: could not send EOSE")
		return
	}
}

// QueryEvents queries events from the MongoDB collection based on filters
func QueryEvents(filters []relay.Filter, client *mongo.Client, databaseName string) ([]relay.Event, error) {
	var results []relay.Event

	for _, filter := range filters {
		filterBson := bson.M{}

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
			filterBson["createdat"] = bson.M{"$gte": *filter.Since}
		}
		if filter.Until != nil {
			if filterBson["createdat"] == nil {
				filterBson["createdat"] = bson.M{"$lte": *filter.Until}
			} else {
				filterBson["createdat"].(bson.M)["$lte"] = *filter.Until
			}
		}

		opts := options.Find().SetSort(bson.D{{Key: "createdat", Value: -1}})
		if filter.Limit != nil {
			opts.SetLimit(int64(*filter.Limit))
		}

		for _, kind := range filter.Kinds {
			collectionName := fmt.Sprintf("event-kind%d", kind)
			collection := client.Database(databaseName).Collection(collectionName)
			cursor, err := collection.Find(context.TODO(), filterBson, opts)
			if err != nil {
				return nil, fmt.Errorf("error querying events: %v", err)
			}
			defer cursor.Close(context.TODO())

			for cursor.Next(context.TODO()) {
				var event relay.Event
				if err := cursor.Decode(&event); err != nil {
					return nil, fmt.Errorf("error decoding event: %v", err)
				}
				results = append(results, event)
			}
			if err := cursor.Err(); err != nil {
				return nil, fmt.Errorf("cursor error: %v", err)
			}
		}
	}

	return results, nil
}

func SendClosed(ws *websocket.Conn, subID string, message string) {
	closeMsg := []interface{}{"CLOSED", subID, message}
	closeBytes, _ := json.Marshal(closeMsg)
	websocket.Message.Send(ws, string(closeBytes))
}
