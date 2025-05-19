package utils

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	relay "github.com/0ceanslim/grain/server/types"
	"golang.org/x/net/websocket"
)

func SendToBackupRelay(backupURL string, evt relay.Event) error {
	conn, err := websocket.Dial(backupURL, "", "http://localhost/")
	if err != nil {
		return fmt.Errorf("error connecting to backup relay %s: %w", backupURL, err)
	}
	defer conn.Close()

	// Create the message to send
	eventMessage := []interface{}{"EVENT", evt}
	eventMessageBytes, err := json.Marshal(eventMessage)
	if err != nil {
		return fmt.Errorf("error marshaling event message: %w", err)
	}

	if _, err := conn.Write(eventMessageBytes); err != nil {
		return fmt.Errorf("error sending event message to backup relay: %w", err)
	}

	// Log and return
	log.Printf("Event %s sent to backup relay %s", evt.ID, backupURL)
	time.Sleep(500 * time.Millisecond) // Optional: small delay to avoid rapid successive sends

	return nil
}
