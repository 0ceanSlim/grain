package response

import (
	nostr "github.com/0ceanslim/grain/server/types"
)

// SendOK sends an "OK" response to the client
func SendOK(client nostr.ClientInterface, eventID string, status bool, message string) {
	response := []interface{}{"OK", eventID, status, message}
	client.SendMessage(response)
}
