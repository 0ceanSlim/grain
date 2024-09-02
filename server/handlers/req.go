package handlers

import (
	"encoding/json"
	"fmt"
	"grain/config"
	"grain/server/db"
	"grain/server/handlers/response"
	relay "grain/server/types"
	"grain/server/utils"
	"sync"

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
		queriedEvents, err := db.QueryEvents(filters, db.GetClient(), "grain")
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
