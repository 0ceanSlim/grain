package core

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
	"golang.org/x/net/websocket"
)

// ConnectionStatus represents the state of a relay connection
type ConnectionStatus int

const (
	StatusDisconnected ConnectionStatus = iota
	StatusConnecting
	StatusConnected
	StatusError
)

// RelayPool manages multiple relay connections
type RelayPool struct {
	connections   map[string]*RelayConnection
	mu            sync.RWMutex
	config        *Config
	messageRouter *MessageRouter
}

// RelayConnection represents a single relay connection
type RelayConnection struct {
	URL           string
	Conn          *websocket.Conn
	Status        ConnectionStatus
	LastPing      time.Time
	Subscriptions map[string]bool
	mu            sync.RWMutex
	writeChan     chan []byte
	done          chan struct{}
	messageRouter *MessageRouter // Add message router
}

// MessageRouter handles routing messages to subscriptions
type MessageRouter struct {
	subscriptions map[string]*Subscription
	mu            sync.RWMutex
}

// NewMessageRouter creates a new message router
func NewMessageRouter() *MessageRouter {
	return &MessageRouter{
		subscriptions: make(map[string]*Subscription),
	}
}

// RegisterSubscription registers a subscription for message routing
func (mr *MessageRouter) RegisterSubscription(subID string, sub *Subscription) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.subscriptions[subID] = sub
}

// UnregisterSubscription removes a subscription from message routing
func (mr *MessageRouter) UnregisterSubscription(subID string) {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	delete(mr.subscriptions, subID)
}

// RouteMessage routes a message to the appropriate subscription
func (mr *MessageRouter) RouteMessage(subID string, messageType string, data interface{}) {
	mr.mu.RLock()
	sub, exists := mr.subscriptions[subID]
	mr.mu.RUnlock()

	if !exists {
		return
	}

	switch messageType {
	case "EVENT":
		if eventData, ok := data.(map[string]interface{}); ok {
			if event := parseEventFromData(eventData); event != nil {
				select {
				case sub.Events <- event:
					log.ClientCore().Debug("Event routed to subscription", "sub_id", subID, "event_id", event.ID)
				default:
					log.ClientCore().Warn("Subscription event channel full", "sub_id", subID)
				}
			}
		}
	case "EOSE":
		select {
		case sub.Done <- struct{}{}:
			log.ClientCore().Debug("EOSE routed to subscription", "sub_id", subID)
		default:
			// EOSE already sent or channel closed
		}
	}
}

// NewRelayPool creates a new relay pool
func NewRelayPool(config *Config) *RelayPool {
	return &RelayPool{
		connections:   make(map[string]*RelayConnection),
		config:        config,
		messageRouter: NewMessageRouter(),
	}
}

// Connect establishes a connection to a relay
func (rp *RelayPool) Connect(url string) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	// Check if already connected
	if conn, exists := rp.connections[url]; exists && conn.Status == StatusConnected {
		log.ClientCore().Debug("Already connected to relay", "relay", url)
		return nil
	}

	log.ClientCore().Debug("Connecting to relay", "relay", url)

	// Create relay connection
	relayConn := &RelayConnection{
		URL:           url,
		Status:        StatusConnecting,
		Subscriptions: make(map[string]bool),
		writeChan:     make(chan []byte, 100),
		done:          make(chan struct{}),
		messageRouter: rp.messageRouter,
	}

	// Attempt WebSocket connection with timeout
	origin := "http://localhost/"

	// Create a custom dialer with timeout
	config, err := websocket.NewConfig(url, origin)
	if err != nil {
		relayConn.Status = StatusError
		log.ClientCore().Error("Failed to create WebSocket config", "relay", url, "error", err)
		return fmt.Errorf("failed to create config for relay %s: %w", url, err)
	}

	// Set connection timeout
	config.Dialer = &net.Dialer{
		Timeout: rp.config.ConnectionTimeout,
	}

	conn, err := websocket.DialConfig(config)
	if err != nil {
		relayConn.Status = StatusError
		log.ClientCore().Error("Failed to connect to relay", "relay", url, "error", err)
		return fmt.Errorf("failed to connect to relay %s: %w", url, err)
	}

	relayConn.Conn = conn
	relayConn.Status = StatusConnected
	relayConn.LastPing = time.Now()

	// Store connection
	rp.connections[url] = relayConn

	// Start connection handlers
	go relayConn.writeHandler()
	go relayConn.readHandler()

	log.ClientCore().Info("Connected to relay", "relay", url)
	return nil
}

// SendMessage sends a message to a specific relay
func (rp *RelayPool) SendMessage(url string, message interface{}) error {
	rp.mu.RLock()
	conn, exists := rp.connections[url]
	rp.mu.RUnlock()

	if !exists || conn.Status != StatusConnected {
		return fmt.Errorf("not connected to relay %s", url)
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	select {
	case conn.writeChan <- data:
		log.ClientCore().Debug("Message queued for relay", "relay", url)
		return nil
	case <-time.After(rp.config.WriteTimeout):
		return fmt.Errorf("timeout sending message to relay %s", url)
	}
}

// BroadcastMessage sends a message to multiple relays
func (rp *RelayPool) BroadcastMessage(message interface{}, urls []string) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	log.ClientCore().Debug("Broadcasting message", "relay_count", len(urls))

	var lastErr error
	sent := 0

	for _, url := range urls {
		rp.mu.RLock()
		conn, exists := rp.connections[url]
		rp.mu.RUnlock()

		if !exists || conn.Status != StatusConnected {
			lastErr = fmt.Errorf("not connected to relay %s", url)
			continue
		}

		select {
		case conn.writeChan <- data:
			sent++
		case <-time.After(rp.config.WriteTimeout):
			lastErr = fmt.Errorf("timeout sending to relay %s", url)
		}
	}

	if sent == 0 && lastErr != nil {
		return lastErr
	}

	log.ClientCore().Debug("Message broadcast complete", "sent", sent, "total", len(urls))
	return nil
}

// GetConnection returns a specific relay connection
func (rp *RelayPool) GetConnection(url string) (*RelayConnection, error) {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	conn, exists := rp.connections[url]
	if !exists {
		return nil, fmt.Errorf("no connection to relay %s", url)
	}

	return conn, nil
}

// GetConnectedRelays returns a list of connected relay URLs
func (rp *RelayPool) GetConnectedRelays() []string {
	rp.mu.RLock()
	defer rp.mu.RUnlock()

	var connected []string
	for url, conn := range rp.connections {
		if conn.Status == StatusConnected {
			connected = append(connected, url)
		}
	}

	return connected
}

// CloseConnection closes a specific relay connection
func (rp *RelayPool) CloseConnection(url string) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	conn, exists := rp.connections[url]
	if !exists {
		return fmt.Errorf("no connection to relay %s", url)
	}

	return conn.close()
}

// RegisterSubscription registers a subscription for message routing
func (rp *RelayPool) RegisterSubscription(subID string, sub *Subscription) {
	rp.messageRouter.RegisterSubscription(subID, sub)
}

// UnregisterSubscription removes a subscription from message routing
func (rp *RelayPool) UnregisterSubscription(subID string) {
	rp.messageRouter.UnregisterSubscription(subID)
}

// Close shuts down all relay connections
func (rp *RelayPool) Close() error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	log.ClientCore().Info("Closing relay pool", "connection_count", len(rp.connections))

	for url, conn := range rp.connections {
		if err := conn.close(); err != nil {
			log.ClientCore().Error("Error closing relay connection", "relay", url, "error", err)
		}
	}

	rp.connections = make(map[string]*RelayConnection)
	return nil
}

// writeHandler manages outgoing messages for a relay connection
func (rc *RelayConnection) writeHandler() {
	defer func() {
		if rc.Conn != nil {
			rc.Conn.Close()
		}
	}()

	for {
		select {
		case data := <-rc.writeChan:
			if err := websocket.Message.Send(rc.Conn, string(data)); err != nil {
				log.ClientCore().Error("Failed to send message to relay", "relay", rc.URL, "error", err)
				rc.Status = StatusError
				return
			}
			log.ClientCore().Debug("Message sent to relay", "relay", rc.URL)

		case <-rc.done:
			log.ClientCore().Debug("Write handler stopped", "relay", rc.URL)
			return
		}
	}
}

// readHandler manages incoming messages from a relay connection
func (rc *RelayConnection) readHandler() {
	defer func() {
		if rc.Conn != nil {
			rc.Conn.Close()
		}
		rc.Status = StatusDisconnected
		log.ClientCore().Debug("Read handler terminated", "relay", rc.URL)
	}()

	for {
		select {
		case <-rc.done:
			log.ClientCore().Debug("Read handler stopped", "relay", rc.URL)
			return
		default:
			// Set a longer read timeout to avoid frequent timeouts
			rc.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))

			var message string
			if err := websocket.Message.Receive(rc.Conn, &message); err != nil {
				// Don't log timeout errors as errors - they're normal for keep-alive
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					log.ClientCore().Debug("Read timeout from relay (normal keep-alive)", "relay", rc.URL)
					continue // Continue loop, don't terminate connection
				}

				log.ClientCore().Warn("Failed to read message from relay", "relay", rc.URL, "error", err)
				rc.Status = StatusError
				return
			}

			// Process the received message
			if err := rc.processMessage(message); err != nil {
				log.ClientCore().Warn("Failed to process message from relay", "relay", rc.URL, "error", err)
			}
		}
	}
}

// close terminates a relay connection
func (rc *RelayConnection) close() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.Status == StatusDisconnected {
		return nil
	}

	log.ClientCore().Debug("Closing relay connection", "relay", rc.URL)

	close(rc.done)

	if rc.Conn != nil {
		if err := rc.Conn.Close(); err != nil {
			log.ClientCore().Error("Error closing WebSocket connection", "relay", rc.URL, "error", err)
			return err
		}
	}

	rc.Status = StatusDisconnected
	log.ClientCore().Debug("Relay connection closed", "relay", rc.URL)
	return nil
}

// processMessage handles incoming messages from the relay
func (rc *RelayConnection) processMessage(message string) error {
	log.ClientCore().Debug("Processing message from relay", "relay", rc.URL, "message_length", len(message))

	// Parse the message as JSON array
	var messageArray []interface{}
	if err := json.Unmarshal([]byte(message), &messageArray); err != nil {
		return fmt.Errorf("invalid message format: %w", err)
	}

	if len(messageArray) == 0 {
		return fmt.Errorf("empty message")
	}

	messageType, ok := messageArray[0].(string)
	if !ok {
		return fmt.Errorf("invalid message type")
	}

	switch messageType {
	case "EVENT":
		if len(messageArray) >= 3 {
			subID, ok := messageArray[1].(string)
			if !ok {
				return fmt.Errorf("invalid subscription ID in EVENT")
			}

			eventData, ok := messageArray[2].(map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid event data in EVENT")
			}

			log.ClientCore().Debug("Received EVENT message", "relay", rc.URL, "sub_id", subID)
			rc.messageRouter.RouteMessage(subID, "EVENT", eventData)
		}
	case "EOSE":
		if len(messageArray) >= 2 {
			subID, ok := messageArray[1].(string)
			if !ok {
				return fmt.Errorf("invalid subscription ID in EOSE")
			}

			log.ClientCore().Debug("Received EOSE message", "relay", rc.URL, "sub_id", subID)
			rc.messageRouter.RouteMessage(subID, "EOSE", nil)
		}
	case "CLOSED":
		if len(messageArray) >= 2 {
			subID, ok := messageArray[1].(string)
			if !ok {
				return fmt.Errorf("invalid subscription ID in CLOSED")
			}

			log.ClientCore().Debug("Received CLOSED message", "relay", rc.URL, "sub_id", subID)
			rc.messageRouter.RouteMessage(subID, "CLOSED", nil)
		}
	case "NOTICE":
		if len(messageArray) >= 2 {
			notice, ok := messageArray[1].(string)
			if !ok {
				notice = "unknown notice"
			}
			log.ClientCore().Info("Relay notice", "relay", rc.URL, "notice", notice)
		}
	case "OK":
		if len(messageArray) >= 3 {
			log.ClientCore().Debug("Received OK message", "relay", rc.URL)
			// TODO: Handle event publication response
		}
	default:
		log.ClientCore().Debug("Unknown message type", "relay", rc.URL, "type", messageType)
	}

	return nil
}

// parseEventFromData converts message data to a nostr.Event
func parseEventFromData(data map[string]interface{}) *nostr.Event {
	// Convert the map to JSON and back to parse properly
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.ClientCore().Error("Failed to marshal event data", "error", err)
		return nil
	}

	var event nostr.Event
	if err := json.Unmarshal(jsonData, &event); err != nil {
		log.ClientCore().Error("Failed to unmarshal event", "error", err)
		return nil
	}

	return &event
}

// ping sends a ping to keep the connection alive (if needed)
func (rc *RelayConnection) ping() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.Status != StatusConnected {
		return fmt.Errorf("connection not active")
	}

	// Most Nostr relays don't require explicit pings, but we update the timestamp
	rc.LastPing = time.Now()
	return nil
}
