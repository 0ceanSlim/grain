package negentropy

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/gorilla/websocket"

	configTypes "grain/config/types"
	nostr "grain/server/types"
)

// storeUserRelays forwards the event to the local relay via WebSocket.
func storeUserRelays(event nostr.Event, serverCfg *configTypes.ServerConfig) error {
	// Construct the WebSocket URL for the local relay
	localRelayURL := fmt.Sprintf("ws://localhost%s", serverCfg.Server.Port)

	// Connect to the local relay WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(localRelayURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to local relay WebSocket: %w", err)
	}
	defer conn.Close()

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
