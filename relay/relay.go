package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"grain/relay/db"
	"grain/relay/kinds"
	relay "grain/relay/types"
	"grain/relay/utils"

	"golang.org/x/net/websocket"
)

var subscriptions = make(map[string]relay.Subscription)

func Listener(ws *websocket.Conn) {
	defer ws.Close()

	var msg string
	for {
		err := websocket.Message.Receive(ws, &msg)
		if err != nil {
			fmt.Println("Error receiving message:", err)
			return
		}
		fmt.Println("Received message:", msg)

		var message []interface{}
		err = json.Unmarshal([]byte(msg), &message)
		if err != nil {
			fmt.Println("Error parsing message:", err)
			continue
		}

		if len(message) < 2 {
			fmt.Println("Invalid message format")
			continue
		}

		messageType, ok := message[0].(string)
		if !ok {
			fmt.Println("Invalid message type")
			continue
		}

		switch messageType {
		case "EVENT":
			handleEvent(ws, message)
		case "REQ":
			handleReq(ws, message)
		case "CLOSE":
			handleClose(ws, message)
		default:
			fmt.Println("Unknown message type:", messageType)
		}
	}
}

func handleEvent(ws *websocket.Conn, message []interface{}) {
	if len(message) != 2 {
		fmt.Println("Invalid EVENT message format")
		return
	}

	eventData, ok := message[1].(map[string]interface{})
	if !ok {
		fmt.Println("Invalid event data format")
		return
	}
	eventBytes, err := json.Marshal(eventData)
	if err != nil {
		fmt.Println("Error marshaling event data:", err)
		return
	}

	var evt relay.Event
	err = json.Unmarshal(eventBytes, &evt)
	if err != nil {
		fmt.Println("Error unmarshaling event data:", err)
		return
	}

	// Call the HandleKind function
	HandleKind(context.TODO(), evt, ws)

	fmt.Println("Event processed:", evt.ID)
}

func HandleKind(ctx context.Context, evt relay.Event, ws *websocket.Conn) {
	if !utils.CheckSignature(evt) {
		sendOKResponse(ws, evt.ID, false, "invalid: signature verification failed")
		return
	}

	collection := db.GetCollection(evt.Kind)

	var err error
	switch evt.Kind {
	case 0:
		err = kinds.HandleKind0(ctx, evt, collection)
	case 1:
		err = kinds.HandleKind1(ctx, evt, collection)
	default:
		err = kinds.HandleUnknownKind(ctx, evt, collection)
	}

	if err != nil {
		sendOKResponse(ws, evt.ID, false, fmt.Sprintf("error: %v", err))
		return
	}

	sendOKResponse(ws, evt.ID, true, "")
}

func sendOKResponse(ws *websocket.Conn, eventID string, status bool, message string) {
	response := []interface{}{"OK", eventID, status, message}
	responseBytes, _ := json.Marshal(response)
	websocket.Message.Send(ws, string(responseBytes))
}

func handleReq(ws *websocket.Conn, message []interface{}) {
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

func handleClose(ws *websocket.Conn, message []interface{}) {
	if len(message) != 2 {
		fmt.Println("Invalid CLOSE message format")
		return
	}

	subID, ok := message[1].(string)
	if !ok {
		fmt.Println("Invalid subscription ID format")
		return
	}

	delete(subscriptions, subID)
	fmt.Println("Subscription closed:", subID)

	closeMsg := []interface{}{"CLOSED", subID, "Subscription closed"}
	closeBytes, _ := json.Marshal(closeMsg)
	err := websocket.Message.Send(ws, string(closeBytes))
	if err != nil {
		fmt.Println("Error sending CLOSE message:", err)
		return
	}
}
