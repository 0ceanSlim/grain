package handlers

import (
	"github.com/0ceanslim/grain/server/handlers/response"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// HandleClose processes a "CLOSE" message from a client. A valid CLOSE gets
// no reply: NIP-01 reserves CLOSED for subscriptions the relay ends itself,
// and clients log a CLOSED they didn't expect as an error. A malformed CLOSE
// has no usable sub id to answer on, so it gets a NOTICE.
func HandleClose(client nostr.ClientInterface, message []interface{}) {
	if len(message) != 2 {
		log.Close().Debug("Invalid CLOSE message format", "message_length", len(message))
		if client.IsConnected() {
			response.SendNotice(client, "invalid: CLOSE must be [\"CLOSE\", <subscription_id>]")
		}
		return
	}

	subID, ok := message[1].(string)
	if !ok || len(subID) == 0 || len(subID) > 64 {
		log.Close().Debug("Invalid subscription ID in CLOSE message", "sub_id", subID)
		if client.IsConnected() {
			response.SendNotice(client, "invalid: CLOSE subscription ID must be a string of 1-64 characters")
		}
		return
	}

	// Closing an unknown sub is a no-op, not an error — typically a sub the
	// relay already ended (and announced with CLOSED) or a client double-close.
	if _, exists := client.GetSubscriptions()[subID]; !exists {
		log.Close().Debug("Attempted to close non-existent subscription",
			"subscription_id", subID,
			"active_subscriptions", client.SubscriptionCount())
		return
	}

	client.DeleteSubscription(subID)
	log.Close().Info("Subscription closed by client request",
		"subscription_id", subID,
		"remaining_subscriptions", client.SubscriptionCount())
}
