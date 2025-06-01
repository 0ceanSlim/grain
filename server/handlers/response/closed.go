package response

import (
	nostr "github.com/0ceanslim/grain/server/types"
)

// SendClosed sends a "CLOSED" response to the client
func SendClosed(client nostr.ClientInterface, subID string, message string) {
	closeMsg := []interface{}{"CLOSED", subID, message}
	client.SendMessage(closeMsg)
}
