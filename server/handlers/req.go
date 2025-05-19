package handlers

import (
	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/db/mongo"
	"github.com/0ceanslim/grain/server/handlers/response"
	relay "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
)

// Package-level logger for request handler
var reqLog = utils.GetLogger("req-handler")

// HandleReq processes a new subscription request
func HandleReq(client relay.ClientInterface, message []interface{}) {
	if len(message) < 3 {
		reqLog.Error("Invalid REQ message format")
		response.SendClosed(client, "", "invalid: invalid REQ message format")
		return
	}

	subscriptions := client.GetSubscriptions()

	subID, ok := message[1].(string)
	if !ok || len(subID) == 0 || len(subID) > 64 {
		reqLog.Error("Invalid subscription ID format or length", 
			"sub_id", subID, 
			"length", len(subID))
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
		reqLog.Info("Dropped oldest subscription", 
			"old_sub_id", oldestSubID, 
			"current_count", len(subscriptions))
	}

	// Parse and validate filters
	filters := make([]relay.Filter, len(message)-2)
	for i, filter := range message[2:] {
		filterData, ok := filter.(map[string]interface{})
		if !ok {
			reqLog.Error("Invalid filter format", 
				"sub_id", subID, 
				"filter_index", i)
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
	reqLog.Info("Subscription updated", 
		"sub_id", subID, 
		"filter_count", len(filters), 
		"total_subscriptions", len(subscriptions))

	// Query database
	dbName := config.GetConfig().MongoDB.Database
	queriedEvents, err := mongo.QueryEvents(filters, mongo.GetClient(), dbName)
	if err != nil {
		reqLog.Error("Error querying events", 
			"sub_id", subID, 
			"database", dbName, 
			"error", err)
		response.SendClosed(client, subID, "error: could not query events")
		return
	}

	// Log event count for debugging and monitoring purposes
	reqLog.Debug("Events retrieved from database", 
		"sub_id", subID, 
		"event_count", len(queriedEvents))

	// Send events to client
	for _, evt := range queriedEvents {
		client.SendMessage([]interface{}{"EVENT", subID, evt})
	}

	// Send EOSE message
	client.SendMessage([]interface{}{"EOSE", subID})

	reqLog.Info("Subscription handling completed", 
		"sub_id", subID, 
		"events_sent", len(queriedEvents))
}