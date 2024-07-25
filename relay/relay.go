package relay

import (
	"encoding/json"
	"fmt"
	"grain/relay/handlers"

	"golang.org/x/net/websocket"
)

func WebSocketHandler(ws *websocket.Conn) {
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
			handlers.HandleEvent(ws, message)
		case "REQ":
			handlers.HandleReq(ws, message)
		case "CLOSE":
			handlers.HandleClose(ws, message)
		default:
			fmt.Println("Unknown message type:", messageType)
		}
	}
}
