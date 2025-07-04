package handlers

import (
	"github.com/0ceanslim/grain/server/handlers/response"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// HandleClose processes a "CLOSE" message from a client
func HandleClose(client nostr.ClientInterface, message []interface{}) {
	if len(message) != 2 {
		log.Close().Debug("Invalid CLOSE message format", "message_length", len(message))
		// Only send response if client is still connected
		if client.IsConnected() {
			response.SendClosed(client, "", "invalid: invalid CLOSE message format")
		}
		return
	}

	subID, ok := message[1].(string)
	if !ok {
		log.Close().Debug("Invalid subscription ID format in CLOSE message")
		// Only send response if client is still connected
		if client.IsConnected() {
			response.SendClosed(client, "", "invalid: subscription ID must be a string")
		}
		return
	}

	// Validate subscription ID length (as per Nostr spec)
	if len(subID) == 0 || len(subID) > 64 {
		log.Close().Debug("Invalid subscription ID length",
			"sub_id", subID,
			"length", len(subID))
		// Only send response if client is still connected
		if client.IsConnected() {
			response.SendClosed(client, subID, "invalid: subscription ID must be between 1 and 64 characters")
		}
		return
	}

	// Get the client's subscription map
	subscriptions := client.GetSubscriptions()

	// Check if subscription exists before removing
	if _, exists := subscriptions[subID]; !exists {
		// Use DEBUG since this can happen in normal operation
		// (e.g., client sends duplicate CLOSE, network issues, race conditions)
		log.Close().Debug("Attempted to close non-existent subscription",
			"subscription_id", subID,
			"active_subscriptions", len(subscriptions),
			"client_connected", client.IsConnected())
		// Only send response if client is still connected
		if client.IsConnected() {
			response.SendClosed(client, subID, "subscription was not active")
		}
		return
	}

	// Remove the subscription
	delete(subscriptions, subID)
	log.Close().Info("Subscription closed by client request",
		"subscription_id", subID,
		"remaining_subscriptions", len(subscriptions),
		"client_connected", client.IsConnected())

	// Only send CLOSED response if client is still connected
	if client.IsConnected() {
		response.SendClosed(client, subID, "subscription closed")
	} else {
		log.Close().Debug("Skipping CLOSED response - client disconnected",
			"subscription_id", subID)
	}
}
