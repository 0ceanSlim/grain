package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/handlers"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"

	"golang.org/x/net/websocket"
)

// Set the logging component for client connections
func clientLog() *slog.Logger {
	return utils.GetLogger("client")
}


// Client implements ClientInterface
type Client struct {
	ws            *websocket.Conn
	subscriptions map[string][]nostr.Filter
	rateLimiter   *config.RateLimiter
	messageBuffer strings.Builder

	// Debugging Information
	id 			string
	ip          string
	userAgent   string
	origin      string
	connectedAt time.Time

	// Message monitoring
	messagesSent    int64
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
		
		clientLog().Info("Connection and message statistics", 
			"messages_sent", sent,
			"active_connections", currentConnections,
			"memory_used_pct", memStats["memory_used_percent"],
			"estimated_mem_per_conn_mb", memStats["estimated_mem_per_conn_mb"])
	}
}

func ClientHandler(ws *websocket.Conn) {
	// Capture client info
	ip := utils.GetClientIP(ws.Request())
	userAgent := ws.Request().Header.Get("User-Agent")
	origin := ws.Request().Header.Get("Origin")

	// Create a new client without buffer
	client := &Client{
		ws:            ws,
		subscriptions: make(map[string][]nostr.Filter),
		rateLimiter:   config.GetRateLimiter(),
		messageBuffer: strings.Builder{},
		id:            fmt.Sprintf("c%d", time.Now().UnixNano()),
		ip:            ip,
		userAgent:     userAgent,
		origin:        origin,
		connectedAt:   time.Now(),
	}

	clientsMu.Lock()
	clients[ws] = client
	currentConnections++
	clientsMu.Unlock()

	// Register with connection manager
	connManager.RegisterConnection(client)

	clientLog().Info("New connection established", 
		"client_id", client.id,
		"ip", ip, 
		"user_agent", userAgent,
		"connections", currentConnections)

	// Start processing incoming messages
	clientReader(client)
}

// Implement `ClientInterface` methods
// SendMessage sends a message directly to the WebSocket
func (c *Client) SendMessage(msg interface{}) {
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		clientLog().Error("Failed to marshal message", "error", err)
		return
	}
	
	// Send message directly to WebSocket
	err = websocket.Message.Send(c.ws, string(jsonMsg))
	if err != nil {
		clientLog().Warn("Failed to send message directly",
			"error", err,
			"client_id", c.id)
			
		// Close the connection on send failure
		c.CloseClient()
		return
	}
	
	// Update statistics
	atomic.AddInt64(&c.messagesSent, 1)
	atomic.AddInt64(&totalMessagesSent, 1)
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

func (c *Client) CloseClient() {
	clientsMu.Lock()
	_, exists := clients[c.ws]
	if exists {
		delete(clients, c.ws)
		currentConnections--
	}
	clientsMu.Unlock()

	// Unregister from connection manager
	connManager.RemoveConnection(c)

	// Safely close WebSocket
	c.ws.Close()
}

func clientReader(client *Client) {
	ws := client.ws
	defer func() {
		clientsMu.Lock()
		delete(clients, ws)
		clientsMu.Unlock()

		mu.Lock()
		currentConnections--
		mu.Unlock()

		// Unregister from connection manager
		connManager.RemoveConnection(client)

		client.CloseClient()
	}()

	for {
		var chunk string
		err := websocket.Message.Receive(ws, &chunk)
		if err != nil {
			handleReadError(err, ws)
			return
		}

		client.messageBuffer.WriteString(chunk)
		fullMessage := client.messageBuffer.String()

		if !isValidJSON(fullMessage) {
			clientLog().Debug("Waiting for full JSON message", 
			"client_id", client.id,
			"client", client.ClientInfo())
			continue
		}

		client.messageBuffer.Reset()

		var message []interface{}
		err = json.Unmarshal([]byte(fullMessage), &message)
		if err != nil {
			clientLog().Error("JSON parse error", 
			"error", err, 
			"client_id", client.id,
			"client", client.ClientInfo())
			continue
		}

		if len(message) == 0 {
			clientLog().Warn("Empty message received", "client", client.ClientInfo())
			continue
		}

		messageType, ok := message[0].(string)
		if !ok {
			clientLog().Warn("Invalid message type", "client", client.ClientInfo())
			continue
		}

		switch messageType {
		case "REQ":
			clientLog().Debug("Processing REQ message", 
				"client_id", client.id,
				"message_length", len(message))
			handlers.HandleReq(client, message)
		case "CLOSE":
			clientLog().Info("Processing CLOSE message", 
				"client_id", client.id,
				"sub_id", func() string {
					if len(message) > 1 {
						if subID, ok := message[1].(string); ok {
							return subID
						}
					}
					return "unknown"
				}())
			handlers.HandleClose(client, message)
		case "AUTH":
			if config.GetConfig().Auth.Enabled {
				clientLog().Debug("Processing AUTH message", "client_id", client.id)
				handlers.HandleAuth(client, message)
			} else {
				clientLog().Warn("Received AUTH message, but AUTH is disabled", "client", client.ClientInfo())
			}
		case "EVENT":
			clientLog().Debug("Processing EVENT message", 
				"client_id", client.id,
				"message_length", len(message))
			handlers.HandleEvent(client, message)
		default:
			clientLog().Warn("Unknown message type", 
				"type", messageType, 
				"client_id", client.id,
				"full_message", fullMessage[:min(200, len(fullMessage))] + "...")
		}
	}
}

func handleReadError(err error, ws *websocket.Conn) {
    clientsMu.Lock()
    client, exists := clients[ws]
    clientsMu.Unlock()

    clientInfo := "Unknown Client"
    clientID := "unknown"
    if exists {
        clientInfo = client.ClientInfo()
        clientID = client.id
    }

    if errors.Is(err, io.EOF) {
        clientLog().Info("Client disconnected", 
            "client_id", clientID,
            "client", clientInfo)
    } else {
        clientLog().Error("WebSocket read error", 
            "error", err, 
            "client_id", clientID,
            "client", clientInfo)
    }

    // Avoid closing WebSocket multiple times
    if exists {
        client.CloseClient()
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