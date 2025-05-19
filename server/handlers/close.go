package handlers

import (
	"log/slog"

	"github.com/0ceanslim/grain/server/handlers/response"
	relay "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
)

// Set the logging component for CLOSE handler
func closeLog() *slog.Logger {
	return utils.GetLogger("close-handler")
}

// HandleClose processes a "CLOSE" message from a client
func HandleClose(client relay.ClientInterface, message []interface{}) {
	if len(message) != 2 {
		closeLog().Debug("Invalid CLOSE message format")
		return
	}

	subID, ok := message[1].(string)
	if !ok {
		closeLog().Debug("Invalid subscription ID format")
		return
	}

	// Get the client's subscription map
	subscriptions := client.GetSubscriptions()

	// Remove the subscription
	delete(subscriptions, subID)
	closeLog().Debug("Subscription closed", "subscription_id", subID)

	// Send "CLOSED" response to client
	response.SendClosed(client, subID, "Subscription closed")
}