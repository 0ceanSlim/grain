package userSync

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	cfgType "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"golang.org/x/net/websocket"
)

// fetchHaves queries the local relay for events by the user.
func fetchHaves(pubKey, localRelayURL string, syncConfig cfgType.UserSyncConfig) ([]nostr.Event, error) {
	log.UserSync().Debug("Connecting to local relay for haves",
		"relay_url", localRelayURL,
		"pubkey", pubKey)

	conn, err := websocket.Dial(localRelayURL, "", "http://localhost/")
	if err != nil {
		log.UserSync().Error("Failed to connect to local relay",
			"error", err,
			"relay_url", localRelayURL)
		return nil, fmt.Errorf("failed to connect to local relay: %w", err)
	}
	defer conn.Close()

	filter := generateUserSyncFilter(pubKey, syncConfig)
	subRequest := []interface{}{"REQ", "sub_local", filter}
	requestJSON, err := json.Marshal(subRequest)
	if err != nil {
		log.UserSync().Error("Failed to marshal subscription request", "error", err)
		return nil, fmt.Errorf("failed to marshal subscription request: %w", err)
	}

	err = websocket.Message.Send(conn, string(requestJSON))
	if err != nil {
		log.UserSync().Error("Failed to send subscription request", "error", err)
		return nil, fmt.Errorf("failed to send subscription request: %w", err)
	}

	var localEvents []nostr.Event
	eoseReceived := false

	// Set up channels for message handling
	msgChan := make(chan string)
	errChan := make(chan error)

	// Goroutine for reading WebSocket messages
	go func() {
		defer close(msgChan)
		defer close(errChan)
		for {
			var message string
			err := websocket.Message.Receive(conn, &message)
			if err != nil {
				if err == io.EOF {
					log.UserSync().Debug("Local relay connection closed")
					return
				}
				errChan <- err
				return
			}
			msgChan <- message
		}
	}()

	for !eoseReceived {
		select {
		case message, ok := <-msgChan:
			if !ok {
				log.UserSync().Debug("Message channel closed")
				eoseReceived = true
				break
			}

			var response []interface{}
			if err := json.Unmarshal([]byte(message), &response); err != nil {
				log.UserSync().Error("Failed to unmarshal response",
					"error", err,
					"raw_message", message)
				continue
			}

			if len(response) > 0 {
				switch response[0] {
				case "EVENT":
					var event nostr.Event

					eventMap, ok := response[2].(map[string]interface{})
					if !ok {
						log.UserSync().Error("Unexpected event format in haves",
							"event_data", response[2],
							"data_type", fmt.Sprintf("%T", response[2]))
						continue
					}

					eventJSON, err := json.Marshal(eventMap)
					if err != nil {
						log.UserSync().Error("Failed to marshal event in haves", "error", err)
						continue
					}

					err = json.Unmarshal(eventJSON, &event)
					if err != nil {
						log.UserSync().Error("Failed to parse event in haves", "error", err)
						continue
					}

					log.UserSync().Debug("Received local event",
						"event_id", event.ID,
						"kind", event.Kind,
						"created_at", event.CreatedAt)
					localEvents = append(localEvents, event)

				case "EOSE":
					log.UserSync().Debug("EOSE received from local relay")
					eoseReceived = true
				}
			}

		case <-time.After(WebSocketTimeout):
			log.UserSync().Warn("Timeout waiting for local relay response",
				"timeout_seconds", int(WebSocketTimeout.Seconds()))
			eoseReceived = true

		case err, ok := <-errChan:
			if !ok {
				log.UserSync().Debug("Error channel closed")
				eoseReceived = true
				break
			}
			log.UserSync().Error("Error reading from local relay", "error", err)
			eoseReceived = true
		}
	}

	log.UserSync().Info("Finished fetching local events",
		"pubkey", pubKey,
		"total_events", len(localEvents))

	// Send close message
	closeMsg := `["CLOSE", "sub_local"]`
	_ = websocket.Message.Send(conn, closeMsg)

	return localEvents, nil
}
