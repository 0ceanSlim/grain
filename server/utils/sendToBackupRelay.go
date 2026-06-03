package utils

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
	"golang.org/x/net/websocket"
)

const (
	// backupDialTimeout bounds how long a single backup-relay forward waits
	// to establish its connection. Without it, a black-holed backup relay
	// blocks the forwarding goroutine (and the concurrency slot it holds in
	// handlers/event.go) indefinitely — the leak in #95.
	backupDialTimeout = 5 * time.Second
	// backupWriteTimeout bounds the wire write for the same reason.
	backupWriteTimeout = 5 * time.Second
)

// SendToBackupRelay forwards a single event to one backup relay. It is
// best-effort and bounded in time: both the dial and the write are deadlined
// so a slow or unreachable backup can't pin the calling goroutine. Concurrency
// across events is capped by the caller (see handlers/event.go), and a fresh
// connection is used per call. See #95.
func SendToBackupRelay(backupURL string, evt nostr.Event) error {
	log.Util().Debug("Connecting to backup relay",
		"relay_url", backupURL,
		"event_id", evt.ID,
		"event_kind", evt.Kind)

	// Build the dial config explicitly so we can attach a dial timeout —
	// the bare websocket.Dial has none.
	wsCfg, err := websocket.NewConfig(backupURL, "http://localhost/")
	if err != nil {
		log.Util().Error("Failed to build backup relay ws config",
			"relay_url", backupURL,
			"event_id", evt.ID,
			"error", err)
		return fmt.Errorf("error building ws config for backup relay %s: %w", backupURL, err)
	}
	wsCfg.Dialer = &net.Dialer{Timeout: backupDialTimeout}

	conn, err := websocket.DialConfig(wsCfg)
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

	conn.SetWriteDeadline(time.Now().Add(backupWriteTimeout))
	if _, err := conn.Write(eventMessageBytes); err != nil {
		log.Util().Error("Failed to send event to backup relay",
			"relay_url", backupURL,
			"event_id", evt.ID,
			"error", err)
		return fmt.Errorf("error sending event message to backup relay: %w", err)
	}

	log.Util().Info("Event successfully sent to backup relay",
		"relay_url", backupURL,
		"event_id", evt.ID,
		"event_kind", evt.Kind,
		"pubkey", evt.PubKey)

	return nil
}
