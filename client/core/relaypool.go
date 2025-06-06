// Replace client/core/relaypool.go with this session-based approach:

package core

import (
	"encoding/json"
	"fmt"
	"sync"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
	"golang.org/x/net/websocket"
)

// RelayConnection represents a persistent connection to a relay
type RelayConnection struct {
	URL    string
	conn   *websocket.Conn
	mu     sync.RWMutex
	subs   map[string]chan nostr.Event // subscription ID -> event channel
	active bool
}

// SessionRelayPool manages persistent connections for a specific user session
type SessionRelayPool struct {
	sessionID string
	relays    map[string]*RelayConnection
	mu        sync.RWMutex
}

// NewSessionRelayPool creates a new relay pool for a user session
func NewSessionRelayPool(sessionID string) *SessionRelayPool {
	return &SessionRelayPool{
		sessionID: sessionID,
		relays:    make(map[string]*RelayConnection),
	}
}

// Connect establishes a connection to a relay for this session
func (srp *SessionRelayPool) Connect(relayURL string) error {
	srp.mu.Lock()
	defer srp.mu.Unlock()

	// Check if already connected
	if relay, exists := srp.relays[relayURL]; exists && relay.active {
		log.Util().Debug("Already connected to relay", 
			"session", srp.sessionID,
			"relay", relayURL)
		return nil
	}

	log.Util().Info("Connecting to relay", 
		"session", srp.sessionID,
		"relay", relayURL)

	origin := "http://localhost/"
	conn, err := websocket.Dial(relayURL, "", origin)
	if err != nil {
		log.Util().Error("Failed to connect to relay", 
			"session", srp.sessionID,
			"relay", relayURL, 
			"error", err)
		return err
	}

	relay := &RelayConnection{
		URL:    relayURL,
		conn:   conn,
		subs:   make(map[string]chan nostr.Event),
		active: true,
	}

	srp.relays[relayURL] = relay

	// Start message reader goroutine
	go srp.messageReader(relay)

	log.Util().Info("Successfully connected to relay", 
		"session", srp.sessionID,
		"relay", relayURL)
	return nil
}

// Disconnect closes connection to a relay
func (srp *SessionRelayPool) Disconnect(relayURL string) {
	srp.mu.Lock()
	defer srp.mu.Unlock()

	relay, exists := srp.relays[relayURL]
	if !exists {
		return
	}

	relay.mu.Lock()
	relay.active = false
	if relay.conn != nil {
		relay.conn.Close()
	}
	// Close all subscription channels
	for _, ch := range relay.subs {
		close(ch)
	}
	relay.subs = make(map[string]chan nostr.Event)
	relay.mu.Unlock()

	delete(srp.relays, relayURL)
	log.Util().Info("Disconnected from relay", 
		"session", srp.sessionID,
		"relay", relayURL)
}

// DisconnectAll closes all relay connections for this session
func (srp *SessionRelayPool) DisconnectAll() {
	srp.mu.Lock()
	relayURLs := make([]string, 0, len(srp.relays))
	for url := range srp.relays {
		relayURLs = append(relayURLs, url)
	}
	srp.mu.Unlock()

	for _, url := range relayURLs {
		srp.Disconnect(url)
	}

	log.Util().Info("Disconnected from all relays", "session", srp.sessionID)
}

// Subscribe creates a subscription on a specific relay
func (srp *SessionRelayPool) Subscribe(relayURL string, subID string, filter nostr.Filter) (<-chan nostr.Event, error) {
	srp.mu.RLock()
	relay, exists := srp.relays[relayURL]
	srp.mu.RUnlock()

	if !exists || !relay.active {
		return nil, fmt.Errorf("not connected to relay: %s", relayURL)
	}

	relay.mu.Lock()
	defer relay.mu.Unlock()

	// Create event channel for this subscription
	eventChan := make(chan nostr.Event, 100) // Buffered channel
	relay.subs[subID] = eventChan

	// Send REQ message
	req := []interface{}{"REQ", subID, filter}
	reqJSON, err := json.Marshal(req)
	if err != nil {
		delete(relay.subs, subID)
		close(eventChan)
		return nil, err
	}

	if _, err := relay.conn.Write(reqJSON); err != nil {
		delete(relay.subs, subID)
		close(eventChan)
		return nil, err
	}

	log.Util().Debug("Created subscription", 
		"session", srp.sessionID,
		"relay", relayURL, 
		"sub_id", subID)
	return eventChan, nil
}

// Unsubscribe closes a subscription
func (srp *SessionRelayPool) Unsubscribe(relayURL string, subID string) {
	srp.mu.RLock()
	relay, exists := srp.relays[relayURL]
	srp.mu.RUnlock()

	if !exists || !relay.active {
		return
	}

	relay.mu.Lock()
	defer relay.mu.Unlock()

	// Send CLOSE message
	closeReq := []interface{}{"CLOSE", subID}
	closeJSON, _ := json.Marshal(closeReq)
	relay.conn.Write(closeJSON)

	// Close and remove subscription channel
	if ch, exists := relay.subs[subID]; exists {
		close(ch)
		delete(relay.subs, subID)
	}

	log.Util().Debug("Closed subscription", 
		"session", srp.sessionID,
		"relay", relayURL, 
		"sub_id", subID)
}

// PublishEvent publishes an event to a specific relay
func (srp *SessionRelayPool) PublishEvent(relayURL string, event nostr.Event) error {
	srp.mu.RLock()
	relay, exists := srp.relays[relayURL]
	srp.mu.RUnlock()

	if !exists || !relay.active {
		return fmt.Errorf("not connected to relay: %s", relayURL)
	}

	relay.mu.RLock()
	defer relay.mu.RUnlock()

	// Send EVENT message
	eventMsg := []interface{}{"EVENT", event}
	eventJSON, err := json.Marshal(eventMsg)
	if err != nil {
		return err
	}

	_, err = relay.conn.Write(eventJSON)
	return err
}

// ConnectToAll connects to multiple relays for this session
func (srp *SessionRelayPool) ConnectToAll(relayURLs []string) {
	log.Util().Info("Connecting to user relays", 
		"session", srp.sessionID,
		"relay_count", len(relayURLs))

	for _, url := range relayURLs {
		go func(relayURL string) {
			if err := srp.Connect(relayURL); err != nil {
				log.Util().Error("Failed to connect to relay", 
					"session", srp.sessionID,
					"relay", relayURL, 
					"error", err)
			}
		}(url)
	}
}

// GetConnectedRelays returns list of connected relay URLs for this session
func (srp *SessionRelayPool) GetConnectedRelays() []string {
	srp.mu.RLock()
	defer srp.mu.RUnlock()

	var relays []string
	for url, relay := range srp.relays {
		if relay.active {
			relays = append(relays, url)
		}
	}
	return relays
}

// messageReader continuously reads messages from a relay using proper WebSocket message handling
func (srp *SessionRelayPool) messageReader(relay *RelayConnection) {
	defer func() {
		log.Util().Debug("Message reader exiting", 
			"session", srp.sessionID,
			"relay", relay.URL)
		relay.mu.Lock()
		relay.active = false
		relay.mu.Unlock()
	}()

	for relay.active {
		// Use websocket.Message to properly read complete messages
		var messageStr string
		err := websocket.Message.Receive(relay.conn, &messageStr)
		if err != nil {
			if relay.active {
				log.Util().Warn("Error reading from relay", 
					"session", srp.sessionID,
					"relay", relay.URL, 
					"error", err)
			}
			break
		}

		var response []interface{}
		if err := json.Unmarshal([]byte(messageStr), &response); err != nil {
			log.Util().Warn("Failed to parse message", 
				"session", srp.sessionID,
				"relay", relay.URL, 
				"error", err,
				"message_preview", messageStr[:min(100, len(messageStr))])
			continue
		}

		srp.handleMessage(relay, response)
	}
}

// handleMessage processes incoming messages and routes events to appropriate handlers
func (srp *SessionRelayPool) handleMessage(relay *RelayConnection, response []interface{}) {
	if len(response) < 2 {
		return
	}

	switch response[0] {
	case "EVENT":
		srp.handleEventMessage(relay, response)
	case "EOSE":
		srp.handleEOSEMessage(relay, response)
	case "OK":
		srp.handleOKMessage(relay, response)
	case "CLOSED":
		srp.handleClosedMessage(relay, response)
	case "NOTICE":
		srp.handleNoticeMessage(relay, response)
	}
}

// handleEventMessage processes incoming EVENT messages
func (srp *SessionRelayPool) handleEventMessage(relay *RelayConnection, response []interface{}) {
	if len(response) < 3 {
		return
	}

	subID, ok := response[1].(string)
	if !ok {
		return
	}

	// Parse event directly from the interface{} instead of re-marshaling
	eventData, ok := response[2].(map[string]interface{})
	if !ok {
		log.Util().Warn("Invalid event data format", 
			"session", srp.sessionID,
			"relay", relay.URL)
		return
	}

	// Convert to proper Event struct
	event, err := srp.parseEventFromMap(eventData)
	if err != nil {
		log.Util().Warn("Failed to parse event", 
			"session", srp.sessionID,
			"relay", relay.URL, 
			"error", err)
		return
	}

	// Route to subscription channel
	relay.mu.RLock()
	if ch, exists := relay.subs[subID]; exists {
		select {
		case ch <- *event:
		default:
			log.Util().Warn("Subscription channel full, dropping event", 
				"session", srp.sessionID,
				"relay", relay.URL, 
				"sub_id", subID,
				"event_id", event.ID)
		}
	}
	relay.mu.RUnlock()

	// TODO: Add caching logic here for specific event types
	// Example: Cache kind 0 (metadata) and kind 10002 (relay lists)
	if event.Kind == 0 || event.Kind == 10002 {
		// Could cache these events for quick access
		log.Util().Debug("Received cacheable event", 
			"session", srp.sessionID,
			"kind", event.Kind,
			"event_id", event.ID)
	}
}

// parseEventFromMap converts map[string]interface{} to nostr.Event
func (srp *SessionRelayPool) parseEventFromMap(eventData map[string]interface{}) (*nostr.Event, error) {
	event := &nostr.Event{}

	// Parse required fields
	if id, ok := eventData["id"].(string); ok {
		event.ID = id
	}
	if pubkey, ok := eventData["pubkey"].(string); ok {
		event.PubKey = pubkey
	}
	if createdAt, ok := eventData["created_at"].(float64); ok {
		event.CreatedAt = int64(createdAt)
	}
	if kind, ok := eventData["kind"].(float64); ok {
		event.Kind = int(kind)
	}
	if content, ok := eventData["content"].(string); ok {
		event.Content = content
	}
	if sig, ok := eventData["sig"].(string); ok {
		event.Sig = sig
	}

	// Parse tags (array of arrays)
	if tagsInterface, ok := eventData["tags"].([]interface{}); ok {
		for _, tagInterface := range tagsInterface {
			if tagArray, ok := tagInterface.([]interface{}); ok {
				tag := make([]string, len(tagArray))
				for i, item := range tagArray {
					if str, ok := item.(string); ok {
						tag[i] = str
					}
				}
				event.Tags = append(event.Tags, tag)
			}
		}
	}

	return event, nil
}

// handleEOSEMessage processes End of Stored Events messages
func (srp *SessionRelayPool) handleEOSEMessage(relay *RelayConnection, response []interface{}) {
	subID, ok := response[1].(string)
	if ok {
		log.Util().Debug("Received EOSE", 
			"session", srp.sessionID,
			"relay", relay.URL, 
			"sub_id", subID)
	}
}

// handleOKMessage processes command result messages
func (srp *SessionRelayPool) handleOKMessage(relay *RelayConnection, response []interface{}) {
	if len(response) >= 4 {
		eventID, _ := response[1].(string)
		accepted, _ := response[2].(bool)
		message, _ := response[3].(string)
		
		if accepted {
			log.Util().Debug("Event accepted", 
				"session", srp.sessionID,
				"relay", relay.URL, 
				"event_id", eventID)
		} else {
			log.Util().Warn("Event rejected", 
				"session", srp.sessionID,
				"relay", relay.URL, 
				"event_id", eventID, 
				"reason", message)
		}
	}
}

// handleClosedMessage processes subscription closure messages
func (srp *SessionRelayPool) handleClosedMessage(relay *RelayConnection, response []interface{}) {
	subID, _ := response[1].(string)
	reason := ""
	if len(response) > 2 {
		reason, _ = response[2].(string)
	}
	log.Util().Info("Subscription closed by relay", 
		"session", srp.sessionID,
		"relay", relay.URL, 
		"sub_id", subID, 
		"reason", reason)
}

// handleNoticeMessage processes relay notices
func (srp *SessionRelayPool) handleNoticeMessage(relay *RelayConnection, response []interface{}) {
	if len(response) > 1 {
		if notice, ok := response[1].(string); ok {
			log.Util().Info("Relay notice", 
				"session", srp.sessionID,
				"relay", relay.URL, 
				"notice", notice)
		}
	}
}

// Helper function for minimum
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}