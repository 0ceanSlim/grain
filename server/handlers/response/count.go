package response

import (
	nostr "github.com/0ceanslim/grain/server/types"
)

// SendCount sends a NIP-45 COUNT response to the client. When
// approximate is true, the relay signals that the count may not be
// exact (e.g. capped at countHardCap or the result of a multi-filter
// request).
func SendCount(client nostr.ClientInterface, subID string, count int, approximate bool) {
	payload := map[string]interface{}{"count": count}
	if approximate {
		payload["approximate"] = true
	}
	client.SendMessage([]interface{}{"COUNT", subID, payload})
}
