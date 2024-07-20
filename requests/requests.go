package requests

import (
	"context"
	"encoding/json"
	"fmt"

	"grain/events"

	"golang.org/x/net/websocket"
)

func Handler(ws *websocket.Conn) {
	var msg string
	for {
		err := websocket.Message.Receive(ws, &msg)
		if err != nil {
			fmt.Println("Error receiving message:", err)
			return
		}
		fmt.Println("Received message:", msg)

		// Parse the received message
		var event []interface{}
		err = json.Unmarshal([]byte(msg), &event)
		if err != nil {
			fmt.Println("Error parsing message:", err)
			return
		}

		if len(event) < 2 || event[0] != "EVENT" {
			fmt.Println("Invalid event format")
			continue
		}

		// Convert the event map to an Event struct
		eventData, ok := event[1].(map[string]interface{})
		if !ok {
			fmt.Println("Invalid event data format")
			continue
		}
		eventBytes, err := json.Marshal(eventData)
		if err != nil {
			fmt.Println("Error marshaling event data:", err)
			continue
		}

		var evt events.Event
		err = json.Unmarshal(eventBytes, &evt)
		if err != nil {
			fmt.Println("Error unmarshaling event data:", err)
			continue
		}

		err = events.HandleEvent(context.TODO(), evt)
		if err != nil {
			fmt.Println("Error handling event:", err)
			continue
		}

		err = websocket.Message.Send(ws, "Echo: "+msg)
		if err != nil {
			fmt.Println("Error sending message:", err)
			return
		}
	}
}
