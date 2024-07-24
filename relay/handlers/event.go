package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"grain/relay/db"
	"grain/relay/kinds"
	"grain/relay/utils"

	relay "grain/relay/types"

	"golang.org/x/net/websocket"
)

func HandleEvent(ws *websocket.Conn, message []interface{}) {
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
		OKResponse(ws, evt.ID, false, "invalid: signature verification failed")
		return
	}

	collection := db.GetCollection(evt.Kind)

	var err error
	switch {
	case evt.Kind == 0:
		err = kinds.HandleKind0(ctx, evt, collection, ws)
	case evt.Kind == 1:
		err = kinds.HandleKind1(ctx, evt, collection)
	case evt.Kind == 2:
		err = kinds.HandleKind2Deprecated(ctx, evt, ws)
	case evt.Kind == 3:
		err = kinds.HandleReplaceableKind(ctx, evt, collection, ws)
	case evt.Kind >= 4 && evt.Kind < 45:
		err = kinds.HandleRegularKind(ctx, evt, collection)
	case evt.Kind >= 1000 && evt.Kind < 10000:
		err = kinds.HandleRegularKind(ctx, evt, collection)
	case evt.Kind >= 10000 && evt.Kind < 20000:
		err = kinds.HandleReplaceableKind(ctx, evt, collection, ws)
	case evt.Kind >= 20000 && evt.Kind < 30000:
		// Ephemeral events are not stored
		fmt.Println("Ephemeral event received and ignored:", evt.ID)
	case evt.Kind >= 30000 && evt.Kind < 40000:
		err = kinds.HandleParameterizedReplaceableKind(ctx, evt, collection, ws)
	default:
		err = kinds.HandleUnknownKind(ctx, evt, collection)
	}

	if err != nil {
		OKResponse(ws, evt.ID, false, fmt.Sprintf("error: %v", err))
		return
	}

	OKResponse(ws, evt.ID, true, "")
}

func OKResponse(ws *websocket.Conn, eventID string, status bool, message string) {
	response := []interface{}{"OK", eventID, status, message}
	responseBytes, _ := json.Marshal(response)
	websocket.Message.Send(ws, string(responseBytes))
}
