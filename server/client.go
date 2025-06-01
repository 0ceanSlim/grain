package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/handlers"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"

	"golang.org/x/net/websocket"
)

// Client implements ClientInterface
type Client struct {
	ws            *websocket.Conn
	subscriptions map[string][]nostr.Filter
	rateLimiter   *config.RateLimiter
	messageBuffer strings.Builder

	// Timeout configuration
	readTimeout     time.Duration // Per-message read timeout
	writeTimeout    time.Duration // Per-message write timeout
	idleTimeout     time.Duration // Connection idle timeout
	lastActivity    time.Time     // Last message activity

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Write mutex to prevent concurrent writes
	writeMu sync.Mutex

	// Debugging Information
	id 			string
	ip          string
	userAgent   string
	origin      string
	connectedAt time.Time

	// Message monitoring
	messagesSent    int64
	mu           sync.RWMutex // Protects lastActivity
}

// Track active clients
var (
	currentConnections int
	mu                 sync.Mutex
	clients            = make(map[*websocket.Conn]*Client)
	clientsMu          sync.Mutex
	
	// Global stats
	totalMessagesSent    int64
)

// PrintStats periodically logs messaging and connection statistics
func PrintStats() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sent := atomic.LoadInt64(&totalMessagesSent)
		
		// Reset counters
		atomic.StoreInt64(&totalMessagesSent, 0)
		
		// Get memory statistics from connection manager
		memStats := connManager.GetMemoryStats()
		
		log.Client().Info("Connection and message statistics", 
			"messages_sent", sent,
			"active_connections", currentConnections,
			"memory_used_pct", memStats["memory_used_percent"],
			"estimated_mem_per_conn_mb", memStats["estimated_mem_per_conn_mb"])
	}
}

func ClientHandler(ws *websocket.Conn) {
	cfg := config.GetConfig()
	
	// Create context for this client connection
	ctx, cancel := context.WithCancel(context.Background())

	// Capture client info
	ip := utils.GetClientIP(ws.Request())
	userAgent := ws.Request().Header.Get("User-Agent")
	origin := ws.Request().Header.Get("Origin")

	// Create client with timeout configuration
	client := &Client{
		ws:            ws,
		subscriptions: make(map[string][]nostr.Filter),
		rateLimiter:   config.GetRateLimiter(),
		messageBuffer: strings.Builder{},
		
		// Configure timeouts from config
		readTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		writeTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		idleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
		lastActivity: time.Now(),
		
		ctx:    ctx,
		cancel: cancel,
		
		id:          fmt.Sprintf("c%d", time.Now().UnixNano()),
		ip:          ip,
		userAgent:   userAgent,
		origin:      origin,
		connectedAt: time.Now(),
	}

	clientsMu.Lock()
	clients[ws] = client
	currentConnections++
	clientsMu.Unlock()

	// Register with connection manager
	connManager.RegisterConnection(client)

	log.Client().Info("New connection established", 
		"client_id", client.id,
		"ip", ip, 
		"user_agent", userAgent,
		"read_timeout_sec", cfg.Server.ReadTimeout,
		"write_timeout_sec", cfg.Server.WriteTimeout,
		"idle_timeout_sec", cfg.Server.IdleTimeout,
		"connections", currentConnections)

	// Start idle timeout monitor if configured
	if client.idleTimeout > 0 {
		go client.monitorIdleTimeout()
	}

	// Start processing incoming messages
	clientReader(client)
}

// monitorIdleTimeout checks for idle connections and closes them
func (c *Client) monitorIdleTimeout() {
	// Check every 1/4 of the idle timeout period
	checkInterval := c.idleTimeout / 4
	if checkInterval < 30*time.Second {
		checkInterval = 30 * time.Second // Minimum check interval
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.mu.RLock()
			idle := time.Since(c.lastActivity)
			c.mu.RUnlock()

			if idle > c.idleTimeout {
				log.Client().Info("Closing idle connection", 
					"client_id", c.id,
					"idle_duration_sec", int(idle.Seconds()),
					"idle_timeout_sec", int(c.idleTimeout.Seconds()))
				
				// Send notice before closing (best effort, no timeout)
				c.sendNoticeNoTimeout("Connection closed due to inactivity")
				c.CloseClient()
				return
			}
		}
	}
}

// updateActivity updates the last activity timestamp
func (c *Client) updateActivity() {
	c.mu.Lock()
	c.lastActivity = time.Now()
	c.mu.Unlock()
}

// Implement `ClientInterface` methods
// SendMessage sends a message with write timeout and proper error handling
func (c *Client) SendMessage(msg interface{}) {
	select {
	case <-c.ctx.Done():
		return // Connection is closing
	default:
	}

	// Check if connection is still valid before attempting to send
	if c.ws == nil {
		return
	}

	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		log.Client().Error("Failed to marshal message", 
			"client_id", c.id,
			"error", err)
		return
	}
	
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	// Double-check connection is still valid after acquiring lock
	select {
	case <-c.ctx.Done():
		return
	default:
	}

	// Set write deadline if timeout is configured
	if c.writeTimeout > 0 {
		deadline := time.Now().Add(c.writeTimeout)
		if err := c.ws.SetWriteDeadline(deadline); err != nil {
			// Don't log error if connection is already closed
			if !isConnectionClosed(err) {
				log.Client().Error("Failed to set write deadline", 
					"client_id", c.id,
					"timeout_sec", int(c.writeTimeout.Seconds()),
					"error", err)
			}
			c.markDisconnected()
			c.CloseClient()
			return
		}
	}
	
	// Send message to WebSocket
	err = websocket.Message.Send(c.ws, string(jsonMsg))
	
	// Clear write deadline to prevent it from affecting future operations
	if c.writeTimeout > 0 {
		c.ws.SetWriteDeadline(time.Time{}) // Clear deadline
	}
	
	if err != nil {
		// Mark as disconnected before logging/cleanup
		c.markDisconnected()
		
		// Only log if it's not a connection closed error
		if !isConnectionClosed(err) {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Client().Warn("Write timeout sending message",
					"client_id", c.id,
					"timeout_sec", int(c.writeTimeout.Seconds()))
			} else {
				log.Client().Warn("Failed to send message",
					"error", err,
					"client_id", c.id,
					"error_type", fmt.Sprintf("%T", err))
			}
		}
		c.CloseClient()
		return
	}
	
	// Update activity and statistics
	c.updateActivity()
	atomic.AddInt64(&c.messagesSent, 1)
	atomic.AddInt64(&totalMessagesSent, 1)
}

// IsConnected returns true if the client connection is still active
func (c *Client) IsConnected() bool {
	select {
	case <-c.ctx.Done():
		return false
	default:
		return true
	}
}

// markDisconnected marks the client as disconnected (internal use)
func (c *Client) markDisconnected() {
	c.cancel() // This will make IsConnected() return false
}

// Helper function to check if error is due to closed connection
func isConnectionClosed(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "broken pipe") ||
		   strings.Contains(errStr, "connection reset") ||
		   strings.Contains(errStr, "use of closed network connection") ||
		   strings.Contains(errStr, "connection refused")
}

// sendNoticeNoTimeout sends a notice without timeout (for cleanup scenarios)
func (c *Client) sendNoticeNoTimeout(message string) {
	notice := []interface{}{"NOTICE", message}
	jsonMsg, err := json.Marshal(notice)
	if err != nil {
		return // Best effort
	}
	
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	
	// No timeout for cleanup messages
	websocket.Message.Send(c.ws, string(jsonMsg))
}

func (c *Client) GetWS() *websocket.Conn {
	return c.ws
}

func (c *Client) ClientInfo() string {
	return fmt.Sprintf(
		"Client Info - ID: %s, IP: %s, User-Agent: %s, Origin: %s, Connected At: %s, Active Subscriptions: %d",
		c.id,
		c.ip,
		c.userAgent,
		c.origin,
		c.connectedAt.Format(time.RFC3339),
		len(c.subscriptions),
	)
}

func (c *Client) GetSubscriptions() map[string][]nostr.Filter {
	return c.subscriptions
}

// CloseClient closes the client connection and cleans up resources
func (c *Client) CloseClient() {
	// Cancel context first to stop all goroutines
	c.cancel()
	
	// Remove from client tracking
	clientsMu.Lock()
	_, exists := clients[c.ws]
	if exists {
		delete(clients, c.ws)
		currentConnections--
	}
	clientsMu.Unlock()

	// Only proceed if we were actually tracking this client
	if !exists {
		return
	}

	// Unregister from connection manager
	connManager.RemoveConnection(c)

	// Clear all subscriptions
	for subID := range c.subscriptions {
		delete(c.subscriptions, subID)
	}

	// Close WebSocket connection
	if c.ws != nil {
		c.ws.Close()
	}

	log.Client().Debug("Client connection closed and cleaned up", 
		"client_id", c.id,
		"remaining_connections", currentConnections)
}

// clientReader reads messages from the WebSocket connection and processes them
func clientReader(client *Client) {
	ws := client.ws
	defer func() {
		// Mark as disconnected first
		client.markDisconnected()
		
		// Clean up tracking when function exits
		clientsMu.Lock()
		delete(clients, ws)
		mu.Lock()
		currentConnections--
		mu.Unlock()
		clientsMu.Unlock()

		// Unregister from connection manager
		connManager.RemoveConnection(client)
		
		// Close the connection if not already closed
		ws.Close()
		
		log.Client().Debug("Client reader exited and connection cleaned up", 
			"client_id", client.id,
			"remaining_connections", currentConnections)
	}()

	for {
		// Check if context is cancelled
		select {
		case <-client.ctx.Done():
			log.Client().Debug("Client reader stopping due to context cancellation", 
				"client_id", client.id)
			return
		default:
		}

		// Set read deadline if timeout is configured
		if client.readTimeout > 0 {
			deadline := time.Now().Add(client.readTimeout)
			if err := ws.SetReadDeadline(deadline); err != nil {
				if !isConnectionClosed(err) {
					log.Client().Error("Failed to set read deadline", 
						"client_id", client.id,
						"timeout_sec", int(client.readTimeout.Seconds()),
						"error", err)
				}
				return
			}
		}

		var chunk string
		err := websocket.Message.Receive(ws, &chunk)
		
		// Clear read deadline after operation
		if client.readTimeout > 0 {
			ws.SetReadDeadline(time.Time{}) // Clear deadline
		}
		
		if err != nil {
			handleReadError(err, client)
			return
		}

		rateLimiter := config.GetRateLimiter()
		if rateLimiter != nil {
			if allowed, msg := rateLimiter.AllowWs(); !allowed {
				log.Client().Warn("WebSocket rate limit exceeded", 
					"client_id", client.id,
					"reason", msg)
				
				// Send notice and close connection due to rate limiting
				client.sendNoticeNoTimeout("rate-limited: " + msg)
				return
			}
		}
		
		// Update activity on successful message receive
		client.updateActivity()

		client.messageBuffer.WriteString(chunk)
		fullMessage := client.messageBuffer.String()
		cfg := config.GetConfig()

		if !isValidJSON(fullMessage) {
			// Use configurable max event size as message buffer limit
			maxEventSize := cfg.RateLimit.MaxEventSize
			if maxEventSize <= 0 {
				maxEventSize = 1024 * 1024 // Default to 1MB if not configured
			}
			
			if client.messageBuffer.Len() > maxEventSize {
				log.Client().Warn("Message buffer exceeds max event size, closing connection", 
					"client_id", client.id,
					"buffer_size", client.messageBuffer.Len(),
					"max_event_size", maxEventSize)
				return
			}
			log.Client().Debug("Waiting for full JSON message", 
				"client_id", client.id,
				"buffer_size", len(fullMessage),
				"max_allowed", maxEventSize)
			continue
		}

		client.messageBuffer.Reset()

		var message []interface{}
		err = json.Unmarshal([]byte(fullMessage), &message)
		if err != nil {
			log.Client().Error("JSON parse error", 
				"error", err, 
				"client_id", client.id,
				"message_length", len(fullMessage))
			continue
		}

		if len(message) == 0 {
			log.Client().Warn("Empty message received", 
				"client_id", client.id)
			continue
		}

		messageType, ok := message[0].(string)
		if !ok {
			log.Client().Warn("Invalid message type", 
				"client_id", client.id,
				"message_type", fmt.Sprintf("%T", message[0]))
			continue
		}

		// Process message based on type
		switch messageType {
		case "REQ":
			log.Client().Debug("Processing REQ message", 
				"client_id", client.id,
				"message_parts", len(message))
			handlers.HandleReq(client, message)
		case "CLOSE":
			subID := "unknown"
			if len(message) > 1 {
				if id, ok := message[1].(string); ok {
					subID = id
				}
			}
			log.Client().Debug("Processing CLOSE message", 
				"client_id", client.id,
				"sub_id", subID)
			handlers.HandleClose(client, message)
		case "AUTH":
			if config.GetConfig().Auth.Enabled {
				log.Client().Debug("Processing AUTH message", 
					"client_id", client.id)
				handlers.HandleAuth(client, message)
			} else {
				log.Client().Warn("Received AUTH message, but AUTH is disabled", 
					"client_id", client.id)
			}
		case "EVENT":
			log.Client().Debug("Processing EVENT message", 
				"client_id", client.id,
				"message_parts", len(message))
			handlers.HandleEvent(client, message)
		default:
			log.Client().Warn("Unknown message type", 
				"type", messageType, 
				"client_id", client.id,
				"message_preview", func() string {
					if len(fullMessage) > 200 {
						return fullMessage[:200] + "..."
					}
					return fullMessage
				}())
		}
	}
}

// handleReadError handles errors that occur during reading from the WebSocket connection
func handleReadError(err error, client *Client) {
	clientID := client.id
	
	// Determine error type and log appropriately
	if errors.Is(err, io.EOF) {
		log.Client().Info("Client disconnected normally", 
			"client_id", clientID)
	} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		log.Client().Info("Client read timeout", 
			"client_id", clientID,
			"timeout_sec", int(client.readTimeout.Seconds()))
	} else if isConnectionClosed(err) {
		log.Client().Debug("Connection closed during read", 
			"client_id", clientID)
	} else {
		log.Client().Error("WebSocket read error", 
			"error", err, 
			"client_id", clientID,
			"error_type", fmt.Sprintf("%T", err))
	}
}

func isValidJSON(s string) bool {
	var js interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

// Start stats monitoring
func InitStatsMonitoring() {
	go PrintStats()
}