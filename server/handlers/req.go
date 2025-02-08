package handlers

import (
	"encoding/json"
	"fmt"
	"grain/config"
	"grain/server/db/mongo"
	"grain/server/handlers/response"
	relay "grain/server/types"
	"grain/server/utils"
	"sync"

	"golang.org/x/net/websocket"
)

var subscriptions = make(map[string]relay.Subscription)
var mu sync.Mutex // Protect concurrent access to subscriptions map

// RequestQueue holds incoming requests
var RequestQueue = make(chan Request, 1000) // Adjust buffer size as needed

type Request struct {
	Ws      *websocket.Conn
	Message []interface{}
}

// StartWorkerPool initializes a pool of worker goroutines
func StartWorkerPool(numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		go worker()
	}
}

// worker processes requests from the RequestQueue
func worker() {
	for req := range RequestQueue {
		processRequest(req.Ws, req.Message)
	}
}

// HandleReq now just adds the request to the queue
func HandleReq(ws *websocket.Conn, message []interface{}, subscriptions map[string][]relay.Filter) {
	select {
	case RequestQueue <- Request{Ws: ws, Message: message}:
		// Request added to queue
	default:
		// Queue is full, log the dropped request
		fmt.Println("Warning: Request queue is full, dropping request")
	}
}

// processRequest handles the actual processing of each request
func processRequest(ws *websocket.Conn, message []interface{}) {
	if len(message) < 3 {
		fmt.Println("Invalid REQ message format")
		response.SendClosed(ws, "", "invalid: invalid REQ message format")
		return
	}

	subID, ok := message[1].(string)
	if !ok || len(subID) == 0 || len(subID) > 64 {
		fmt.Println("Invalid subscription ID format or length")
		response.SendClosed(ws, "", "invalid: subscription ID must be between 1 and 64 characters long")
		return
	}

	mu.Lock()
	defer mu.Unlock()

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

	// Validate filters
	if !utils.ValidateFilters(filters) {
		fmt.Println("Invalid filters: hex values not valid")
		response.SendClosed(ws, subID, "invalid: filters contain invalid hex values")
		return
	}

	// Add subscription
	subscriptions[subID] = relay.Subscription{Filters: filters}
	fmt.Printf("Subscription updated: %s with %d filters\n", subID, len(filters))

	// ✅ Get database name dynamically from config
	dbName := config.GetConfig().MongoDB.Database

	// Query the database with filters and send back the results
	queriedEvents, err := mongo.QueryEvents(filters, mongo.GetClient(), dbName)
	if err != nil {
		fmt.Println("Error querying events:", err)
		response.SendClosed(ws, subID, "error: could not query events")
		return
	}

	fmt.Printf("Retrieved %d events for subscription %s\n", len(queriedEvents), subID)
	if len(queriedEvents) == 0 {
		fmt.Printf("No matching events found for subscription %s\n", subID)
	}

	for _, evt := range queriedEvents {
		msg := []interface{}{"EVENT", subID, evt}
		msgBytes, _ := json.Marshal(msg)

		if err := websocket.Message.Send(ws, string(msgBytes)); err != nil {
			fmt.Println("Client disconnected, stopping event send.")
			response.SendClosed(ws, subID, "error: client disconnected")
			return // Stop sending further events
		}
	}

	// Send EOSE message
	eoseMsg := []interface{}{"EOSE", subID}
	eoseBytes, _ := json.Marshal(eoseMsg)
	err = websocket.Message.Send(ws, string(eoseBytes))
	if err != nil {
		fmt.Println("Error sending EOSE:", err)
		response.SendClosed(ws, subID, "error: could not send EOSE")
		return
	}

	fmt.Println("Subscription handling completed, keeping connection open.")
}

// Initialize the worker pool when your server starts
func init() {
	StartWorkerPool(10) // Adjust the number of workers as needed
}
