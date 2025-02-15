package handlers

import (
	"fmt"
	"grain/server/handlers/response"
	relay "grain/server/types"
)

// HandleClose processes a "CLOSE" message from a client
func HandleClose(client relay.ClientInterface, message []interface{}) {
	if len(message) != 2 {
		fmt.Println("Invalid CLOSE message format")
		return
	}

	subID, ok := message[1].(string)
	if !ok {
		fmt.Println("Invalid subscription ID format")
		return
	}

	// Get the client's subscription map
	subscriptions := client.GetSubscriptions()

	// Remove the subscription
	delete(subscriptions, subID)
	fmt.Println("Subscription closed:", subID)

	// Send "CLOSED" response to client
	response.SendClosed(client, subID, "Subscription closed")
}
