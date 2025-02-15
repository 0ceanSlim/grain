package handlers

import (
	"fmt"
	"grain/config"
	"grain/server/db/mongo"
	"grain/server/handlers/response"
	relay "grain/server/types"
	"grain/server/utils"
)

// HandleReq processes a new subscription request
func HandleReq(client relay.ClientInterface, message []interface{}) {
	if len(message) < 3 {
		fmt.Println("Invalid REQ message format")
		response.SendClosed(client, "", "invalid: invalid REQ message format")
		return
	}

	subscriptions := client.GetSubscriptions()

	subID, ok := message[1].(string)
	if !ok || len(subID) == 0 || len(subID) > 64 {
		fmt.Println("Invalid subscription ID format or length")
		response.SendClosed(client, "", "invalid: subscription ID must be between 1 and 64 characters long")
		return
	}

	// Remove oldest subscription if needed
	if len(subscriptions) >= config.GetConfig().Server.MaxSubscriptionsPerClient {
		var oldestSubID string
		for id := range subscriptions {
			oldestSubID = id
			break
		}
		delete(subscriptions, oldestSubID)
		fmt.Println("Dropped oldest subscription:", oldestSubID)
	}

	// Parse and validate filters
	filters := make([]relay.Filter, len(message)-2)
	for i, filter := range message[2:] {
		filterData, ok := filter.(map[string]interface{})
		if !ok {
			fmt.Println("Invalid filter format")
			response.SendClosed(client, subID, "invalid: invalid filter format")
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

	// Add subscription
	subscriptions[subID] = filters
	fmt.Printf("Subscription updated: %s with %d filters\n", subID, len(filters))

	// Query database
	dbName := config.GetConfig().MongoDB.Database
	queriedEvents, err := mongo.QueryEvents(filters, mongo.GetClient(), dbName)
	if err != nil {
		fmt.Println("Error querying events:", err)
		response.SendClosed(client, subID, "error: could not query events")
		return
	}

	for _, evt := range queriedEvents {
		client.SendMessage([]interface{}{"EVENT", subID, evt})
	}

	// Send EOSE message
	client.SendMessage([]interface{}{"EOSE", subID})

	fmt.Println("Subscription handling completed.")
}
