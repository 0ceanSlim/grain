package response

import (
	nostr "github.com/0ceanslim/grain/server/types"
)

// SendNotice sends a NIP-01 ["NOTICE", <message>] to the client
func SendNotice(client nostr.ClientInterface, message string) {
	notice := []interface{}{"NOTICE", message}
	client.SendMessage(notice)
}
