package response

import (
	relay "grain/server/types"
)

// SendNotice sends a notice message to the client
func SendNotice(client relay.ClientInterface, pubKey, message string) {
	notice := []interface{}{"NOTICE", pubKey, message}
	client.SendMessage(notice)
}