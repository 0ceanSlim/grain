package core

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

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
	connections map[string]*RelayConnection
	mu          sync.RWMutex
	config      *Config
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
}

// NewRelayPool creates a new relay pool
func NewRelayPool(config *Config) *RelayPool {
	return &RelayPool{
		connections: make(map[string]*RelayConnection),
		config:      config,
	}
}

// Connect establishes a connection to a relay
func (rp *RelayPool) Connect(url string) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	
	// Check if already connected
	if conn, exists := rp.connections[url]; exists && conn.Status == StatusConnected {
		log.Util().Debug("Already connected to relay", "relay", url)
		return nil
	}
	
	log.Util().Debug("Connecting to relay", "relay", url)
	
	// Create relay connection
	relayConn := &RelayConnection{
		URL:           url,
		Status:        StatusConnecting,
		Subscriptions: make(map[string]bool),
		writeChan:     make(chan []byte, 100),
		done:          make(chan struct{}),
	}
	
	// Attempt WebSocket connection
	origin := "http://localhost/"
	conn, err := websocket.Dial(url, "", origin)
	if err != nil {
		relayConn.Status = StatusError
		log.Util().Error("Failed to connect to relay", "relay", url, "error", err)
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
	
	log.Util().Info("Connected to relay", "relay", url)
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
		log.Util().Debug("Message queued for relay", "relay", url)
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
	
	log.Util().Debug("Broadcasting message", "relay_count", len(urls))
	
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
	
	log.Util().Debug("Message broadcast complete", "sent", sent, "total", len(urls))
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

// Close shuts down all relay connections
func (rp *RelayPool) Close() error {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	
	log.Util().Info("Closing relay pool", "connection_count", len(rp.connections))
	
	for url, conn := range rp.connections {
		if err := conn.close(); err != nil {
			log.Util().Error("Error closing relay connection", "relay", url, "error", err)
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
				log.Util().Error("Failed to send message to relay", "relay", rc.URL, "error", err)
				rc.Status = StatusError
				return
			}
			log.Util().Debug("Message sent to relay", "relay", rc.URL)
			
		case <-rc.done:
			log.Util().Debug("Write handler stopped", "relay", rc.URL)
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
	}()
	
	for {
		select {
		case <-rc.done:
			log.Util().Debug("Read handler stopped", "relay", rc.URL)
			return
		default:
			// Set read timeout
			rc.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
			
			var message string
			if err := websocket.Message.Receive(rc.Conn, &message); err != nil {
				log.Util().Error("Failed to read message from relay", "relay", rc.URL, "error", err)
				rc.Status = StatusError
				return
			}
			
			log.Util().Debug("Message received from relay", "relay", rc.URL)
			// TODO: Process received message (will be handled by subscription system)
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
	
	log.Util().Debug("Closing relay connection", "relay", rc.URL)
	
	close(rc.done)
	
	if rc.Conn != nil {
		if err := rc.Conn.Close(); err != nil {
			log.Util().Error("Error closing WebSocket connection", "relay", rc.URL, "error", err)
			return err
		}
	}
	
	rc.Status = StatusDisconnected
	log.Util().Debug("Relay connection closed", "relay", rc.URL)
	return nil
}