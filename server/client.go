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

	// Outbound message queue. The dedicated writeLoop goroutine drains
	// this channel and is the ONLY thing that writes to ws. Callers
	// enqueue via SendMessage and never block on the network — a slow
	// or dead peer fills the buffer and gets disconnected via the
	// slow-consumer path rather than freezing the broadcaster.
	//
	// Capacity 256 absorbs bursts (a few seconds of typical event flow)
	// while still detecting truly stuck peers in bounded time.
	outgoing chan []byte

	// Timeout configuration
	readTimeout  time.Duration // Per-message read timeout
	writeTimeout time.Duration // Per-message write timeout
	idleTimeout  time.Duration // Connection idle timeout
	lastActivity time.Time     // Last message activity

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Subscription mutex to protect concurrent subscription access
	subMu sync.RWMutex

	// Debugging Information
	id          string
	ip          string
	userAgent   string
	origin      string
	connectedAt time.Time

	// Message monitoring
	messagesSent int64
	mu           sync.RWMutex // Protects lastActivity
}

// clientOutgoingBuffer is the per-client outbound queue capacity. 256 messages
// is enough to absorb normal bursts; once full the consumer is treated as
// slow/dead and the connection is closed (see SendMessage).
const clientOutgoingBuffer = 256

// Track active clients.
//
// currentConnections is an atomic counter. Map mutations to `clients` and
// the matching counter delta are performed together under clientsMu so the
// counter cannot disagree with map membership. The atomic type lets the
// max-connections gate and the periodic stats reader sample the count
// without grabbing clientsMu.
//
// There are two cleanup paths for a client (CloseClient and the
// clientReader defer). Both race; whichever wins the clientsMu first
// performs the decrement, the other observes the missing map entry and
// becomes a no-op. Prior to issue #67's fix the reader defer
// unconditionally decremented after CloseClient had already done so,
// driving currentConnections deeply negative in production
// (active_connections=-1339 observed) and silently disabling the
// server.max_connections admission gate at client.go:108.
var (
	currentConnections atomic.Int64
	clients            = make(map[*websocket.Conn]*Client)
	clientsMu          sync.Mutex

	// Global stats
	totalMessagesSent int64
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

		log.RelayClient().Info("Connection and message statistics",
			"messages_sent", sent,
			"active_connections", currentConnections.Load(),
			"memory_used_pct", memStats["memory_used_percent"],
			"estimated_mem_per_conn_mb", memStats["estimated_mem_per_conn_mb"])
	}
}

func ClientHandler(ws *websocket.Conn) {
	cfg := config.GetConfig()

	// Enforce max connections. The per-rejection WARN was replaced in
	// #61 with an aggregator that emits one summary line per minute —
	// production saw 169,272 of these WARNs in 4h, drowning every other
	// signal. The rejected client still gets a NOTICE; only the
	// server-side log is aggregated.
	if maxConn := cfg.Server.MaxConnections; maxConn > 0 {
		if current := currentConnections.Load(); current >= int64(maxConn) {
			RecordRejection("max_conn", utils.GetClientIP(ws.Request()))
			websocket.Message.Send(ws, `["NOTICE","error: server at max capacity, try again later"]`)
			ws.Close()
			return
		}
	}

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
		rateLimiter:   config.NewClientRateLimiter(),
		messageBuffer: strings.Builder{},
		outgoing:      make(chan []byte, clientOutgoingBuffer),

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
	connectionCount := currentConnections.Add(1)
	clientsMu.Unlock()

	// Register with connection manager
	connManager.RegisterConnection(client)

	// Start the dedicated writer goroutine BEFORE any SendMessage call
	// (the AUTH challenge below is the first such call).
	go client.writeLoop()

	log.RelayClient().Info("New connection established",
		"client_id", client.id,
		"ip", ip,
		"user_agent", userAgent,
		"read_timeout_sec", cfg.Server.ReadTimeout,
		"write_timeout_sec", cfg.Server.WriteTimeout,
		"idle_timeout_sec", cfg.Server.IdleTimeout,
		"connections", connectionCount)

	// Always send NIP-42 AUTH challenge
	challenge := utils.GenerateChallenge(32)
	handlers.SetChallengeForConnection(client, challenge)
	client.SendMessage([]interface{}{"AUTH", challenge})

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
				log.RelayClient().Info("Closing idle connection",
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

// SendMessage marshals and enqueues a message for delivery to the client.
// It NEVER blocks on the network — the dedicated writeLoop goroutine drains
// the queue. If the per-client buffer is full, the consumer is treated as
// slow/dead: the message is dropped and the connection is closed. This is
// the load-bearing fix for the broadcaster lockup: a single stuck peer can
// no longer freeze BroadcastEvent for every other client.
func (c *Client) SendMessage(msg interface{}) {
	select {
	case <-c.ctx.Done():
		return // Connection is closing
	default:
	}

	if c.ws == nil {
		return
	}

	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		log.RelayClient().Error("Failed to marshal message",
			"client_id", c.id,
			"error", err)
		return
	}

	select {
	case c.outgoing <- jsonMsg:
		// Enqueued. The writer goroutine will deliver it.
	case <-c.ctx.Done():
		return
	default:
		// Buffer full = slow consumer. Close the connection so we stop
		// trying to deliver to a peer that can't keep up. Done in a
		// goroutine to avoid any chance of deadlock with a caller that
		// holds locks (e.g. clientsMu released before BroadcastEvent
		// iterates, but be defensive).
		log.RelayClient().Warn("Slow consumer detected, closing connection",
			"client_id", c.id,
			"ip", c.ip,
			"buffer_capacity", clientOutgoingBuffer)
		c.markDisconnected()
		go c.CloseClient()
	}
}

// writeLoop is the only goroutine that writes to c.ws. It drains the
// outgoing channel, applies the per-message write deadline, and exits
// when the context is cancelled. Centralising writes removes the need
// for a write mutex and removes the deadlock surface that froze the
// broadcaster when a single peer stalled.
func (c *Client) writeLoop() {
	defer func() {
		// On exit, drain any remaining queued messages without writing
		// so SendMessage callers blocked in `select` cases see ctx.Done.
		// (Best-effort; the channel is GC'd, no need to close it.)
		log.RelayClient().Debug("Write loop exited", "client_id", c.id)
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case jsonMsg, ok := <-c.outgoing:
			if !ok {
				return
			}
			if err := c.writeOne(jsonMsg); err != nil {
				// writeOne already logged + marked disconnected; trigger
				// full close and exit.
				go c.CloseClient()
				return
			}
			// Update stats only on successful delivery.
			c.updateActivity()
			atomic.AddInt64(&c.messagesSent, 1)
			atomic.AddInt64(&totalMessagesSent, 1)
		}
	}
}

// writeOne does the synchronous wire write for a single already-marshalled
// message. Only writeLoop should call this. Returns an error if the write
// failed in a way that should terminate the connection.
func (c *Client) writeOne(jsonMsg []byte) error {
	if c.writeTimeout > 0 {
		deadline := time.Now().Add(c.writeTimeout)
		if err := c.ws.SetWriteDeadline(deadline); err != nil {
			if !isConnectionClosed(err) {
				log.RelayClient().Error("Failed to set write deadline",
					"client_id", c.id,
					"timeout_sec", int(c.writeTimeout.Seconds()),
					"error", err)
			}
			c.markDisconnected()
			return err
		}
	}

	err := websocket.Message.Send(c.ws, string(jsonMsg))

	if c.writeTimeout > 0 {
		c.ws.SetWriteDeadline(time.Time{})
	}

	if err != nil {
		c.markDisconnected()
		if !isConnectionClosed(err) {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.RelayClient().Warn("Write timeout sending message",
					"client_id", c.id,
					"timeout_sec", int(c.writeTimeout.Seconds()))
			} else {
				log.RelayClient().Warn("Failed to send message",
					"error", err,
					"client_id", c.id,
					"error_type", fmt.Sprintf("%T", err))
			}
		}
		return err
	}

	return nil
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

	// Cleanup-path send. By the time this runs the writer goroutine has
	// usually already exited (ctx cancelled), so we write directly here.
	// Use a tight deadline so a dead peer can't hang the cleanup path.
	if c.ws == nil {
		return
	}
	c.ws.SetWriteDeadline(time.Now().Add(time.Second))
	_ = websocket.Message.Send(c.ws, string(jsonMsg))
	c.ws.SetWriteDeadline(time.Time{})
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
		c.SubscriptionCount(),
	)
}

func (c *Client) GetSubscriptions() map[string][]nostr.Filter {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	// Return a shallow copy so callers can't mutate the map without the lock
	cp := make(map[string][]nostr.Filter, len(c.subscriptions))
	for k, v := range c.subscriptions {
		cp[k] = v
	}
	return cp
}

func (c *Client) SetSubscription(subID string, filters []nostr.Filter) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	c.subscriptions[subID] = filters
}

func (c *Client) DeleteSubscription(subID string) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	delete(c.subscriptions, subID)
}

func (c *Client) SubscriptionCount() int {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	return len(c.subscriptions)
}

func (c *Client) ForEachSubscription(fn func(subID string, filters []nostr.Filter)) {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	for subID, filters := range c.subscriptions {
		fn(subID, filters)
	}
}

// AllowReq checks this client's per-connection REQ rate limiter.
func (c *Client) AllowReq() (bool, string) {
	if c.rateLimiter == nil {
		return true, ""
	}
	return c.rateLimiter.AllowReq()
}

// AllowEvent checks this client's per-connection event rate limiter.
func (c *Client) AllowEvent(kind int, category string) (bool, string) {
	if c.rateLimiter == nil {
		return true, ""
	}
	return c.rateLimiter.AllowEvent(kind, category)
}

// CloseClient closes the client connection and cleans up resources.
//
// Both this function and the clientReader defer race to clean up a given
// client — whichever wins clientsMu first performs the decrement and the
// other observes the missing map entry as a no-op. The exists guard is
// load-bearing for #67's fix: without it the same connection's lifecycle
// produced two decrements (once here, once in the reader defer), driving
// currentConnections deeply negative and silently disabling the
// max_connections admission gate.
func (c *Client) CloseClient() {
	// Cancel context first to stop all goroutines
	c.cancel()

	// Remove from client tracking
	clientsMu.Lock()
	_, exists := clients[c.ws]
	var remaining int64
	if exists {
		delete(clients, c.ws)
		remaining = currentConnections.Add(-1)
	}
	clientsMu.Unlock()

	// Only proceed if we were actually tracking this client
	if !exists {
		return
	}

	// Unregister from connection manager
	connManager.RemoveConnection(c)

	// Clear all subscriptions
	c.subMu.Lock()
	for subID := range c.subscriptions {
		delete(c.subscriptions, subID)
	}
	c.subMu.Unlock()

	// Close WebSocket connection
	if c.ws != nil {
		c.ws.Close()
	}

	log.RelayClient().Debug("Client connection closed and cleaned up",
		"client_id", c.id,
		"remaining_connections", remaining)
}

// clientReader reads messages from the WebSocket connection and processes them
func clientReader(client *Client) {
	ws := client.ws
	defer func() {
		// Mark as disconnected first
		client.markDisconnected()

		// Clean up tracking when function exits. The exists check pairs
		// with the same check in CloseClient — see #67. Either path may
		// fire first; the second observes the empty map slot and skips
		// the decrement.
		clientsMu.Lock()
		_, exists := clients[ws]
		var remaining int64
		if exists {
			delete(clients, ws)
			remaining = currentConnections.Add(-1)
		} else {
			remaining = currentConnections.Load()
		}
		clientsMu.Unlock()

		if exists {
			// Unregister from connection manager only if we were the
			// path that cleaned up tracking; otherwise CloseClient
			// already did it.
			connManager.RemoveConnection(client)
		}

		// Close the connection if not already closed (idempotent).
		ws.Close()

		log.RelayClient().Debug("Client reader exited and connection cleaned up",
			"client_id", client.id,
			"remaining_connections", remaining)
	}()

	for {
		// Check if context is cancelled
		select {
		case <-client.ctx.Done():
			log.RelayClient().Debug("Client reader stopping due to context cancellation",
				"client_id", client.id)
			return
		default:
		}

		// Set read deadline if timeout is configured
		if client.readTimeout > 0 {
			deadline := time.Now().Add(client.readTimeout)
			if err := ws.SetReadDeadline(deadline); err != nil {
				if !isConnectionClosed(err) {
					log.RelayClient().Error("Failed to set read deadline",
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

		if client.rateLimiter != nil {
			if allowed, msg := client.rateLimiter.AllowWs(); !allowed {
				log.RelayClient().Warn("WebSocket rate limit exceeded",
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
				log.RelayClient().Warn("Message buffer exceeds max event size, closing connection",
					"client_id", client.id,
					"buffer_size", client.messageBuffer.Len(),
					"max_event_size", maxEventSize)
				return
			}
			log.RelayClient().Debug("Waiting for full JSON message",
				"client_id", client.id,
				"buffer_size", len(fullMessage),
				"max_allowed", maxEventSize)
			continue
		}

		client.messageBuffer.Reset()

		var message []interface{}
		err = json.Unmarshal([]byte(fullMessage), &message)
		if err != nil {
			log.RelayClient().Error("JSON parse error",
				"error", err,
				"client_id", client.id,
				"message_length", len(fullMessage))
			continue
		}

		if len(message) == 0 {
			log.RelayClient().Warn("Empty message received",
				"client_id", client.id)
			continue
		}

		messageType, ok := message[0].(string)
		if !ok {
			log.RelayClient().Warn("Invalid message type",
				"client_id", client.id,
				"message_type", fmt.Sprintf("%T", message[0]))
			continue
		}

		// Process message based on type
		switch messageType {
		case "REQ":
			log.RelayClient().Debug("Processing REQ message",
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
			log.RelayClient().Debug("Processing CLOSE message",
				"client_id", client.id,
				"sub_id", subID)
			handlers.HandleClose(client, message)
		case "AUTH":
			log.RelayClient().Debug("Processing AUTH message",
				"client_id", client.id)
			handlers.HandleAuth(client, message)
		case "EVENT":
			log.RelayClient().Debug("Processing EVENT message",
				"client_id", client.id,
				"message_parts", len(message))
			handlers.HandleEvent(client, message)
		default:
			log.RelayClient().Warn("Unknown message type",
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
		log.RelayClient().Info("Client disconnected normally",
			"client_id", clientID)
	} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		log.RelayClient().Info("Client read timeout",
			"client_id", clientID,
			"timeout_sec", int(client.readTimeout.Seconds()))
	} else if isConnectionClosed(err) {
		log.RelayClient().Debug("Connection closed during read",
			"client_id", clientID)
	} else {
		log.RelayClient().Error("WebSocket read error",
			"error", err,
			"client_id", clientID,
			"error_type", fmt.Sprintf("%T", err))
	}
}

func isValidJSON(s string) bool {
	var js interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

// BroadcastEvent sends an event to all connected clients whose active
// subscriptions match the event. This is the real-time delivery mechanism
// required by NIP-01: after EOSE, new matching events are pushed to subscribers.
func BroadcastEvent(evt nostr.Event) {
	clientsMu.Lock()
	snapshot := make([]*Client, 0, len(clients))
	for _, c := range clients {
		snapshot = append(snapshot, c)
	}
	clientsMu.Unlock()

	for _, c := range snapshot {
		if !c.IsConnected() {
			continue
		}
		c.ForEachSubscription(func(subID string, filters []nostr.Filter) {
			for _, f := range filters {
				if f.MatchesEvent(evt) {
					c.SendMessage([]interface{}{"EVENT", subID, evt})
					break
				}
			}
		})
	}
}

// Start stats monitoring
func InitStatsMonitoring() {
	go PrintStats()
	startConnectionRejectionInfrastructure()
}
