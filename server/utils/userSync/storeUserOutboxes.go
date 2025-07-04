package userSync

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"golang.org/x/net/websocket"

	cfgType "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// storeUserOutboxes forwards the event to the local relay via WebSocket with proper connection handling.
func storeUserOutboxes(event nostr.Event, serverCfg *cfgType.ServerConfig) error {
	// Construct the WebSocket URL for the local relay
	localRelayURL := fmt.Sprintf("ws://localhost%s", serverCfg.Server.Port)

	log.UserSync().Debug("Storing outbox event to local relay", 
		"event_id", event.ID,
		"relay_url", localRelayURL,
		"event_kind", event.Kind)

	// Connect to the local relay WebSocket
	conn, err := websocket.Dial(localRelayURL, "", "http://localhost/")
	if err != nil {
		log.UserSync().Error("Failed to connect to local relay WebSocket", 
			"error", err, 
			"relay_url", localRelayURL)
		return fmt.Errorf("failed to connect to local relay WebSocket: %w", err)
	}

	// Ensure graceful closure of the WebSocket connection
	defer func() {
		// Give a brief moment for any pending operations
		time.Sleep(100 * time.Millisecond)
		
		if err := conn.Close(); err != nil {
			log.UserSync().Debug("Error closing WebSocket connection", 
				"error", err,
				"relay_url", localRelayURL)
		} else {
			log.UserSync().Debug("WebSocket connection closed gracefully", 
				"relay_url", localRelayURL)
		}
	}()

	// Create the WebSocket message for the event
	eventMessage := []interface{}{"EVENT", event}

	// Marshal the message to JSON
	messageJSON, err := json.Marshal(eventMessage)
	if err != nil {
		log.UserSync().Error("Failed to marshal WebSocket message", 
			"error", err,
			"event_id", event.ID)
		return fmt.Errorf("failed to marshal WebSocket message: %w", err)
	}

	// Send the event to the local relay
	err = websocket.Message.Send(conn, string(messageJSON))
	if err != nil {
		log.UserSync().Error("Failed to send event to local relay WebSocket", 
			"error", err,
			"event_id", event.ID,
			"relay_url", localRelayURL)
		return fmt.Errorf("failed to send event to local relay WebSocket: %w", err)
	}

	// Wait for response with timeout
	responseChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	go func() {
		var response string
		err := websocket.Message.Receive(conn, &response)
		if err != nil {
			errorChan <- err
			return
		}
		responseChan <- response
	}()

	select {
	case response := <-responseChan:
		// Parse the response to check if event was accepted
		var responseArray []interface{}
		if err := json.Unmarshal([]byte(response), &responseArray); err != nil {
			log.UserSync().Warn("Failed to parse relay response", 
				"error", err,
				"raw_response", response,
				"event_id", event.ID)
		} else if len(responseArray) >= 3 {
			if accepted, ok := responseArray[2].(bool); ok {
				if accepted {
					log.UserSync().Info("Event successfully stored to local relay", 
						"event_id", event.ID,
						"relay_url", localRelayURL)
				} else {
					// Get rejection reason if available
					reason := "unknown"
					if len(responseArray) > 3 {
						if reasonStr, ok := responseArray[3].(string); ok {
							reason = reasonStr
						}
					}
					log.UserSync().Warn("Event rejected by local relay", 
						"event_id", event.ID,
						"reason", reason,
						"relay_url", localRelayURL)
				}
			} else {
				log.UserSync().Warn("Unexpected response format from local relay", 
					"event_id", event.ID,
					"response", response)
			}
		}

	case err := <-errorChan:
		if err == io.EOF {
			log.UserSync().Debug("Local relay connection closed during response wait", 
				"event_id", event.ID)
		} else {
			log.UserSync().Error("Error receiving response from local relay", 
				"error", err,
				"event_id", event.ID)
		}

	case <-time.After(5 * time.Second):
		log.UserSync().Warn("Timeout waiting for local relay response", 
			"event_id", event.ID,
			"relay_url", localRelayURL,
			"timeout_seconds", 5)
	}

	return nil
}