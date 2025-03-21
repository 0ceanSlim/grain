package userSync

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"

	configTypes "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
)

// storeUserOutboxes forwards the event to the local relay via WebSocket and ensures graceful closure.
func storeUserOutboxes(event nostr.Event, serverCfg *configTypes.ServerConfig) error {
	// Construct the WebSocket URL for the local relay
	localRelayURL := fmt.Sprintf("ws://localhost%s", serverCfg.Server.Port)

	// Connect to the local relay WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(localRelayURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to local relay WebSocket: %w", err)
	}

	// Ensure graceful closure of the WebSocket connection
	defer func() {
		// Send a CLOSE frame to the server
		closeMessage := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "closing connection")
		if err := conn.WriteMessage(websocket.CloseMessage, closeMessage); err != nil {
			log.Printf("[ERROR] Failed to send CLOSE message to local relay: %v", err)
		}

		// Wait for server acknowledgment of the CLOSE frame
		conn.SetReadDeadline(time.Now().Add(2 * time.Second)) // Set a 2-second timeout
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[ERROR] Error waiting for server CLOSE acknowledgment: %v", err)
		}

		// Close the connection
		_ = conn.Close()
	}()

	// Create the WebSocket message for the event
	eventMessage := []interface{}{"EVENT", event}

	// Marshal the message to JSON
	messageJSON, err := json.Marshal(eventMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal WebSocket message: %w", err)
	}

	// Send the event to the local relay
	err = conn.WriteMessage(websocket.TextMessage, messageJSON)
	if err != nil {
		return fmt.Errorf("failed to send event to local relay WebSocket: %w", err)
	}

	log.Printf("Event with ID: %s successfully sent to local relay via WebSocket at %s", event.ID, localRelayURL)
	return nil
}
