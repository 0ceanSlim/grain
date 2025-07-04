package utils

import (
	"encoding/json"
	"fmt"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
	"golang.org/x/net/websocket"
)

func SendToBackupRelay(backupURL string, evt nostr.Event) error {
	log.Util().Debug("Connecting to backup relay",
		"relay_url", backupURL,
		"event_id", evt.ID,
		"event_kind", evt.Kind)

	conn, err := websocket.Dial(backupURL, "", "http://localhost/")
	if err != nil {
		log.Util().Error("Failed to connect to backup relay",
			"relay_url", backupURL,
			"event_id", evt.ID,
			"error", err)
		return fmt.Errorf("error connecting to backup relay %s: %w", backupURL, err)
	}
	defer conn.Close()

	// Create the message to send
	eventMessage := []interface{}{"EVENT", evt}
	eventMessageBytes, err := json.Marshal(eventMessage)
	if err != nil {
		log.Util().Error("Failed to marshal event message for backup relay",
			"event_id", evt.ID,
			"error", err)
		return fmt.Errorf("error marshaling event message: %w", err)
	}

	log.Util().Debug("Sending event to backup relay",
		"relay_url", backupURL,
		"event_id", evt.ID,
		"message_size_bytes", len(eventMessageBytes))

	if _, err := conn.Write(eventMessageBytes); err != nil {
		log.Util().Error("Failed to send event to backup relay",
			"relay_url", backupURL,
			"event_id", evt.ID,
			"error", err)
		return fmt.Errorf("error sending event message to backup relay: %w", err)
	}

	// Log success and add small delay
	log.Util().Info("Event successfully sent to backup relay",
		"relay_url", backupURL,
		"event_id", evt.ID,
		"event_kind", evt.Kind,
		"pubkey", evt.PubKey)
	time.Sleep(500 * time.Millisecond) // Small delay to avoid rapid successive sends

	return nil
}
