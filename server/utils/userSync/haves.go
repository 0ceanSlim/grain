package userSync

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	config "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"

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

				eventMap, ok := response[2].(map[string]interface{})
				if !ok {
					log.Printf("[ERROR] Unexpected event format in haves: %+v", response[2])
					continue
				}

				eventJSON, err := json.Marshal(eventMap)
				if err != nil {
					log.Printf("[ERROR] Failed to marshal event in haves: %v", err)
					continue
				}

				err = json.Unmarshal(eventJSON, &event)
				if err != nil {
					log.Printf("[ERROR] Failed to parse event in haves: %v", err)
					continue
				}

				log.Printf("[HAVES] Received event ID: %s", event.ID)
				localEvents = append(localEvents, event)

			case "EOSE":
				log.Printf("[HAVES] Received EOSE from local relay")
				eoseReceived = true
			}
		}
	}

	log.Printf("[HAVES] Total events received: %d", len(localEvents))

	_ = conn.WriteMessage(websocket.TextMessage, []byte(`["CLOSE", "sub_local"]`))
	return localEvents, nil
}
