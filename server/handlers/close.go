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
		closeLog().Debug("Invalid CLOSE message format", "message_length", len(message))
		return
	}

	subID, ok := message[1].(string)
	if !ok {
		closeLog().Debug("Invalid subscription ID format in CLOSE message")
		return
	}

	// Get the client's subscription map
	subscriptions := client.GetSubscriptions()

	// Check if subscription exists before removing
	if _, exists := subscriptions[subID]; !exists {
		closeLog().Warn("Attempted to close non-existent subscription", 
			"subscription_id", subID,
			"active_subscriptions", len(subscriptions))
		return
	}

	// Remove the subscription
	delete(subscriptions, subID)
	closeLog().Info("Subscription closed by client request", 
		"subscription_id", subID,
		"remaining_subscriptions", len(subscriptions))

	// Send "CLOSED" response to client
	response.SendClosed(client, subID, "subscription closed")
}