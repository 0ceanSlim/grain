package response

import (
	relay "grain/server/types"
)

// SendOK sends an "OK" response to the client
func SendOK(client relay.ClientInterface, eventID string, status bool, message string) {
	response := []interface{}{"OK", eventID, status, message}
	client.SendMessage(response)
}
