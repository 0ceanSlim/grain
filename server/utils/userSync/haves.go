package userSync

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	config "grain/config/types"
	nostr "grain/server/types"

	"github.com/gorilla/websocket"
)

// fetchLocalRelayEvents queries the local relay for events by the user.
func fetchHaves(pubKey, localRelayURL string, syncConfig config.UserSyncConfig) ([]nostr.Event, error) {
	log.Printf("Connecting to local relay: %s", localRelayURL)

	conn, _, err := websocket.DefaultDialer.Dial(localRelayURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to local relay: %w", err)
	}
	defer conn.Close()

	filter := generateUserSyncFilter(pubKey, syncConfig)
	subRequest := []interface{}{"REQ", "sub_local", filter}
	requestJSON, _ := json.Marshal(subRequest)

	if err := conn.WriteMessage(websocket.TextMessage, requestJSON); err != nil {
		return nil, fmt.Errorf("failed to send subscription request: %w", err)
	}

	var localEvents []nostr.Event
	eoseReceived := false
	for !eoseReceived {
		conn.SetReadDeadline(time.Now().Add(WebSocketTimeout))
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[ERROR] Reading from local relay: %v", err)
			break
		}

		var response []interface{}
		_ = json.Unmarshal(message, &response)

		if len(response) > 0 {
			switch response[0] {
			case "EVENT":
				var event nostr.Event
				_ = json.Unmarshal([]byte(response[2].(string)), &event)
				localEvents = append(localEvents, event)
			case "EOSE":
				eoseReceived = true
			}
		}
	}

	_ = conn.WriteMessage(websocket.TextMessage, []byte(`["CLOSE", "sub_local"]`))
	return localEvents, nil
}


