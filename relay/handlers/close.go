package handlers

import (
	"encoding/json"
	"fmt"

	"golang.org/x/net/websocket"
)

func HandleClose(ws *websocket.Conn, message []interface{}) {
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
