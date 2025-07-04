package userSync

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"golang.org/x/net/websocket"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// CheckIfUserExistsOnRelay checks if a user exists on the relay by their pubkey.
func CheckIfUserExistsOnRelay(pubKey, eventID string, relays []string) (bool, error) {
	for _, url := range relays {
		log.UserSync().Debug("Checking user existence on relay",
			"pubkey", pubKey,
			"relay_url", url,
			"skip_event_id", eventID)

		// Connect to the relay's WebSocket
		conn, err := websocket.Dial(url, "", "http://localhost/")
		if err != nil {
			log.UserSync().Error("Failed to connect to relay WebSocket",
				"error", err,
				"relay_url", url)
			return false, err
		}
		defer conn.Close()

		// Create a subscription filter to query for events by the pubkey
		filter := nostr.Filter{
			Authors: []string{pubKey}, // Filter by the author (pubkey)
		}

		subID := "sub_check_user"
		subRequest := []interface{}{"REQ", subID, filter}

		requestJSON, err := json.Marshal(subRequest)
		if err != nil {
			log.UserSync().Error("Failed to marshal subscription request", "error", err)
			return false, err
		}

		err = websocket.Message.Send(conn, string(requestJSON))
		if err != nil {
			log.UserSync().Error("Failed to send subscription request",
				"error", err,
				"relay_url", url)
			return false, err
		}

		msgChan := make(chan string)
		errChan := make(chan error)
		eventCount := 0
		isNewUser := true

		go func() {
			defer close(msgChan)
			defer close(errChan)
			for {
				var message string
				err := websocket.Message.Receive(conn, &message)
				if err != nil {
					if err == io.EOF {
						log.UserSync().Debug("WebSocket connection closed", "relay_url", url)
						return
					}
					errChan <- err
					return
				}
				msgChan <- message
			}
		}()

		for {
			select {
			case message, ok := <-msgChan:
				if !ok {
					log.UserSync().Debug("Message channel closed", "relay_url", url)
					return isNewUser, nil
				}

				var response []interface{}
				if err := json.Unmarshal([]byte(message), &response); err != nil {
					log.UserSync().Error("Failed to unmarshal response",
						"error", err,
						"raw_message", message)
					continue
				}

				if len(response) > 0 && response[0] == "EVENT" {
					// Parse the event
					eventData, ok := response[2].(map[string]interface{})
					if !ok {
						log.UserSync().Error("Unexpected event data type",
							"data_type", fmt.Sprintf("%T", response[2]),
							"raw_response", response[2])
						continue
					}

					// Extract the event
					eventBytes, err := json.Marshal(eventData)
					if err != nil {
						log.UserSync().Error("Failed to marshal event data", "error", err)
						continue
					}

					var event nostr.Event
					if err := json.Unmarshal(eventBytes, &event); err != nil {
						log.UserSync().Error("Failed to unmarshal event", "error", err)
						continue
					}

					// Skip the current event being processed
					if event.ID == eventID {
						log.UserSync().Debug("Skipping current event", "event_id", eventID)
						continue
					}

					eventCount++
					isNewUser = false
					log.UserSync().Debug("Found existing event for user",
						"pubkey", pubKey,
						"event_id", event.ID,
						"event_count", eventCount)
				}

				if len(response) > 0 && response[0] == "EOSE" {
					log.UserSync().Debug("EOSE received",
						"relay_url", url,
						"pubkey", pubKey,
						"is_new_user", isNewUser,
						"total_events_found", eventCount)
					return isNewUser, nil
				}

			case <-time.After(WebSocketTimeout):
				log.UserSync().Warn("WebSocket timeout while checking user existence",
					"pubkey", pubKey,
					"relay_url", url,
					"timeout_seconds", int(WebSocketTimeout.Seconds()))
				return isNewUser, nil

			case err, ok := <-errChan:
				if !ok {
					log.UserSync().Debug("Error channel closed", "relay_url", url)
					return isNewUser, nil
				}
				log.UserSync().Error("Error reading from WebSocket",
					"error", err,
					"relay_url", url)
				return false, err
			}
		}
	}
	return true, nil
}
