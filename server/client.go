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
	sendCh        chan string
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
	droppedMessages int
	lastAdjustment  time.Time
	messagesSent    int64
	messagesDropped int64
}

// Track active clients
var (
	currentConnections int
	mu                 sync.Mutex
	clients            = make(map[*websocket.Conn]*Client)
	clientsMu          sync.Mutex
	
	// Global stats
	totalMessagesSent    int64
	totalMessagesDropped int64
)

// PrintStats periodically logs messaging statistics
func PrintStats() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sent := atomic.LoadInt64(&totalMessagesSent)
		dropped := atomic.LoadInt64(&totalMessagesDropped)
		
		// Reset counters
		atomic.StoreInt64(&totalMessagesSent, 0)
		atomic.StoreInt64(&totalMessagesDropped, 0)
		
		// Calculate drop rate
		dropRate := 0.0
		if sent+dropped > 0 {
			dropRate = float64(dropped) / float64(sent+dropped) * 100
		}
		
		clientLog().Info("WebSocket message statistics", 
			"sent", sent, 
			"dropped", dropped, 
			"drop_rate_pct", fmt.Sprintf("%.2f", dropRate),
			"active_connections", currentConnections)
	}
}

func ClientHandler(ws *websocket.Conn) {
	mu.Lock()
	if currentConnections >= config.GetConfig().Server.MaxConnections {
		_ = websocket.Message.Send(ws, `{"error": "too many connections"}`)
		mu.Unlock()
		ws.Close()
		return
	}
	currentConnections++
	mu.Unlock()

	// Capture client info
	ip := utils.GetClientIP(ws.Request())
	userAgent := ws.Request().Header.Get("User-Agent")
	origin := ws.Request().Header.Get("Origin")

	// Get server config
	cfg := config.GetConfig()
	
	// Calculate optimal buffer size based on average message size
	bufferSize := utils.CalculateOptimalBufferSize(cfg)

	// Create a new client with optimized buffer size
	client := &Client{
		ws:            ws,
		sendCh:        make(chan string, bufferSize),
		subscriptions: make(map[string][]nostr.Filter),
		rateLimiter:   config.GetRateLimiter(),
		messageBuffer: strings.Builder{},
		id: fmt.Sprintf("c%d", time.Now().UnixNano()), // Simple unique ID
		ip:            ip,
		userAgent:     userAgent,
		origin:        origin,
		connectedAt:   time.Now(),
		
		// Initialize monitoring fields
		droppedMessages: 0,
		lastAdjustment:  time.Now(),
	}

	clientsMu.Lock()
	clients[ws] = client
	clientsMu.Unlock()

	clientLog().Info("New connection established", 
		"client_id", client.id,
		"ip", ip, 
		"user_agent", userAgent, 
		"buffer_size", bufferSize, 
		"buffer_mb", fmt.Sprintf("%.2f", float64(bufferSize)*float64(utils.BufferMessageSizeLimit)/(1024*1024)))

	// Start goroutine to handle outgoing messages
	go clientWriter(client)

	// Start processing incoming messages
	clientReader(client)
}

// Implement `ClientInterface` methods
func (c *Client) SendMessage(msg interface{}) {
	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		clientLog().Error("Failed to marshal message", "error", err)
		return
	}
	
	// Determine message priority (for logging and potential prioritization)
	priority := "normal"
	if arr, ok := msg.([]interface{}); ok && len(arr) > 0 {
		if msgType, ok := arr[0].(string); ok {
			switch msgType {
			case "NOTICE", "EOSE", "CLOSED", "OK":
				priority = "high"
			}
		}
	}
	
	// Try to send without blocking
	select {
	case c.sendCh <- string(jsonMsg):
		atomic.AddInt64(&c.messagesSent, 1)
		atomic.AddInt64(&totalMessagesSent, 1)
	default:
		c.droppedMessages++
		atomic.AddInt64(&c.messagesDropped, 1)
		atomic.AddInt64(&totalMessagesDropped, 1)
		
		// For high priority messages, attempt to force them through
		if priority == "high" && c.droppedMessages < 1000 {
			select {
			case <-c.sendCh: // Remove one message from the buffer
				// Try again
				select {
				case c.sendCh <- string(jsonMsg):
					clientLog().Debug("Forced high-priority message into buffer", 
						"type", priority,
						"client_id", c.id, 
						"client", c.ClientInfo())
				default:
					clientLog().Warn("Failed to send high-priority message even after making room",
						"client_id", c.id,
						"client", c.ClientInfo())
				}
			default:
				// Couldn't make room
			}
		}
		
		// Only log warnings periodically to prevent log flooding
		if c.droppedMessages == 1 || c.droppedMessages % 100 == 0 {
			clientLog().Warn("Client send buffer full, dropping message", 
				"dropped_count", c.droppedMessages,
				"buffer_size", cap(c.sendCh),
				"buffer_used", len(c.sendCh),
				"client_id", c.id,
				"client", c.ClientInfo())
		}
	}
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
	}
	clientsMu.Unlock()

	// Safely close WebSocket
	c.ws.Close()

	// Prevent closing `sendCh` multiple times
	select {
	case _, ok := <-c.sendCh:
		if !ok {
			// Channel already closed
			return
		}
	default:
		close(c.sendCh)
	}
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
			handlers.HandleReq(client, message)
		case "CLOSE":
			handlers.HandleClose(client, message)
		case "AUTH":
			if config.GetConfig().Auth.Enabled {
				handlers.HandleAuth(client, message)
			} else {
				clientLog().Warn("Received AUTH message, but AUTH is disabled", "client", client.ClientInfo())
			}
		case "EVENT":
			handlers.HandleEvent(client, message)
		default:
			clientLog().Warn("Unknown message type", "type", messageType, "client", client.ClientInfo())
		}
	}
}

func clientWriter(client *Client) {
    ws := client.ws
    
    // Use a ticker for paced sending of messages
    ticker := time.NewTicker(10 * time.Millisecond)
    defer ticker.Stop()
    
    // Track consecutive empty reads for adaptive pacing
    consecEmptyReads := 0
    pacingInterval := 10 * time.Millisecond
    
    // Batch size for sending multiple messages per tick
    const batchSize = 5
    
    // Main loop - using range instead of select
    for range ticker.C {
        // Send up to batchSize messages per tick
        sentInBatch := 0
        
        for i := 0; i < batchSize; i++ {
            select {
            case msg, ok := <-client.sendCh:
                if !ok {
                    // Channel closed, exit
                    return
                }
                
                if err := websocket.Message.Send(ws, msg); err != nil {
                    clientLog().Error("Failed to send message", 
                        "error", err,
                        "client_id", client.id)
                    client.CloseClient()
                    return
                }
                
                sentInBatch++
                consecEmptyReads = 0
                
            default:
                // No more messages available
                consecEmptyReads++
                
                // Adaptive pacing - slow down ticker when queue is empty
                if consecEmptyReads > 50 && pacingInterval < 100*time.Millisecond {
                    // Gradually increase pacing interval
                    pacingInterval += 10 * time.Millisecond
                    ticker.Reset(pacingInterval)
                    
                    clientLog().Debug("Increased pacing interval", 
                        "interval_ms", pacingInterval.Milliseconds(),
                        "client_id", client.id)
                }
                
                // Skip to next ticker iteration instead of using break
                i = batchSize // This forces exit from the inner loop
            }
        }
        
        // If we're actively sending messages, speed up the ticker
        if sentInBatch > 0 && pacingInterval > 10*time.Millisecond {
            pacingInterval = 10 * time.Millisecond
            ticker.Reset(pacingInterval)
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