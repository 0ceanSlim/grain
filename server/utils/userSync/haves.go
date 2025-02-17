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

	// Generate the filter based on UserSyncConfig
	filter := generateUserSyncFilter(pubKey, syncConfig)

	subRequest := []interface{}{
		"REQ",
		"sub_local",
		filter,
	}

	requestJSON, err := json.Marshal(subRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal subscription request: %w", err)
	}

	if err := conn.WriteMessage(websocket.TextMessage, requestJSON); err != nil {
		return nil, fmt.Errorf("failed to send subscription request: %w", err)
	}

	var localEvents []nostr.Event
	for {
		conn.SetReadDeadline(time.Now().Add(WebSocketTimeout))
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[ERROR] Error reading from local relay: %v", err)
			break
		}

		var response []interface{}
		if err := json.Unmarshal(message, &response); err != nil {
			log.Printf("Failed to unmarshal response from local relay: %v", err)
			continue
		}

		if len(response) > 0 {
			switch response[0] {
			case "EVENT":
				var event nostr.Event
				eventData, _ := json.Marshal(response[2])
				if err := json.Unmarshal(eventData, &event); err != nil {
					log.Printf("Failed to parse event from local relay: %v", err)
					continue
				}
				localEvents = append(localEvents, event)

			case "EOSE":
				log.Printf("EOSE received from local relay: %s", localRelayURL)
				return localEvents, nil
			}
		}
	}

	// Close subscription before closing connection
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`["CLOSE", "sub_local"]`))
	return localEvents, nil
}

