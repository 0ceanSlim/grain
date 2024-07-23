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
		return
	}

	subID, ok := message[1].(string)
	if !ok {
		fmt.Println("Invalid subscription ID format")
		return
	}

	filters := make([]relay.Filter, len(message)-2)
	for i, filter := range message[2:] {
		filterData, ok := filter.(map[string]interface{})
		if !ok {
			fmt.Println("Invalid filter format")
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

	subscriptions[subID] = relay.Subscription{ID: subID, Filters: filters}
	fmt.Println("Subscription added:", subID)

	// Query the database with filters and send back the results
	// TO DO why is this taking a certain kind as an argument for collection???
	queriedEvents, err := QueryEvents(filters, db.GetClient(), "grain", "event-kind1")
	if err != nil {
		fmt.Println("Error querying events:", err)
		return
	}

	for _, evt := range queriedEvents {
		msg := []interface{}{"EVENT", subID, evt}
		msgBytes, _ := json.Marshal(msg)
		err = websocket.Message.Send(ws, string(msgBytes))
		if err != nil {
			fmt.Println("Error sending event:", err)
			return
		}
	}

	// Indicate end of stored events
	eoseMsg := []interface{}{"EOSE", subID}
	eoseBytes, _ := json.Marshal(eoseMsg)
	err = websocket.Message.Send(ws, string(eoseBytes))
	if err != nil {
		fmt.Println("Error sending EOSE:", err)
		return
	}
}
// QueryEvents queries events from the MongoDB collection based on filters
func QueryEvents(filters []relay.Filter, client *mongo.Client, databaseName, collectionName string) ([]relay.Event, error) {
	collection := client.Database(databaseName).Collection(collectionName)

	var results []relay.Event

	for _, filter := range filters {
		filterBson := bson.M{}

		if len(filter.IDs) > 0 {
			filterBson["_id"] = bson.M{"$in": filter.IDs}
		}
		if len(filter.Authors) > 0 {
			filterBson["author"] = bson.M{"$in": filter.Authors}
		}
		if len(filter.Kinds) > 0 {
			filterBson["kind"] = bson.M{"$in": filter.Kinds}
		}
		if filter.Tags != nil {
			for key, values := range filter.Tags {
				if len(values) > 0 {
					filterBson[key] = bson.M{"$in": values}
				}
			}
		}
		if filter.Since != nil {
			filterBson["created_at"] = bson.M{"$gte": *filter.Since}
		}
		if filter.Until != nil {
			filterBson["created_at"] = bson.M{"$lte": *filter.Until}
		}

		opts := options.Find()
		if filter.Limit != nil {
			opts.SetLimit(int64(*filter.Limit))
		}

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

	return results, nil
}
