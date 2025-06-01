package response

import (
	nostr "github.com/0ceanslim/grain/server/types"
)

// SendNotice sends a notice message to the client
func SendNotice(client nostr.ClientInterface, pubKey, message string) {
	notice := []interface{}{"NOTICE", pubKey, message}
	client.SendMessage(notice)
}
