package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	//"fmt"
	"grain/config"
	"grain/server/handlers"
	relay "grain/server/types"
	"grain/server/utils"
	"io"
	"log"
	"strings"
	"sync"

	"golang.org/x/net/websocket"
)

// Client implements ClientInterface
type Client struct {
	ws            *websocket.Conn
	sendCh        chan string
	subscriptions map[string][]relay.Filter
	rateLimiter   *config.RateLimiter
	messageBuffer strings.Builder

	// Debugging Information
	ip          string
	userAgent   string
	origin      string
	connectedAt time.Time
}

// Track active clients
var (
	currentConnections int
	mu                 sync.Mutex
	clients            = make(map[*websocket.Conn]*Client)
	clientsMu          sync.Mutex
)

func WebSocketHandler(ws *websocket.Conn) {
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

	log.Printf("New connection: IP=%s, User-Agent=%s, Origin=%s", ip, userAgent, origin)

	// Get resource limits from config
	resourceLimits := config.GetConfig().ResourceLimits

	maxClients := config.GetConfig().Server.MaxConnections
	maxSubs := config.GetConfig().Server.MaxSubscriptionsPerClient
	memoryMBLimit := resourceLimits.MemoryMB
	heapSizeMBLimit := resourceLimits.HeapSizeMB

	// Base buffer size calculation (based on max clients and subs)
	baseBufferSize := maxClients * maxSubs * 2

	// Get current system resource usage
	currentMemoryUsage := utils.GetCurrentMemoryUsageMB()
	currentHeapUsage := utils.GetCurrentHeapUsageMB()

	// Calculate resource usage percentages
	memoryUsagePercent := float64(currentMemoryUsage) / float64(memoryMBLimit)
	heapUsagePercent := float64(currentHeapUsage) / float64(heapSizeMBLimit)

	// Adjust buffer size dynamically based on usage
	scalingFactor := 1.0
	if memoryUsagePercent > 0.75 {
		scalingFactor *= 0.5
	}
	if heapUsagePercent > 0.75 {
		scalingFactor *= 0.5
	}

	// Apply scaling
	dynamicBufferSize := int(float64(baseBufferSize) * scalingFactor)

	// Ensure a reasonable minimum buffer size
	if dynamicBufferSize < 1000 {
		dynamicBufferSize = 1000
	}

	// Create a new client with dynamic buffer size
	client := &Client{
		ws:            ws,
		sendCh:        make(chan string, dynamicBufferSize),
		subscriptions: make(map[string][]relay.Filter),
		rateLimiter:   config.GetRateLimiter(),
		ip:            ip,
		userAgent:     userAgent,
		origin:        origin,
		connectedAt:   time.Now(),
	}

	clientsMu.Lock()
	clients[ws] = client
	clientsMu.Unlock()

	log.Printf("New connection from IP: %s (Buffer Size: %d)", utils.GetClientIP(ws.Request()), dynamicBufferSize)

	// Start goroutine to handle outgoing messages
	go clientWriter(client)

	// Start processing incoming messages
	clientReader(client)
}

// ✅ Implement `ClientInterface` methods
func (c *Client) SendMessage(msg interface{}) {
	jsonMsg, _ := json.Marshal(msg)
	select {
	case c.sendCh <- string(jsonMsg):
	default:
		log.Println("[WARN] Client send buffer full, dropping message")
	}
}

func (c *Client) GetWS() *websocket.Conn {
	return c.ws
}

func (c *Client) ClientInfo() string {
	return fmt.Sprintf(
		"Client Info - IP: %s, User-Agent: %s, Origin: %s, Connected At: %s, Active Subscriptions: %d",
		c.ip,
		c.userAgent,
		c.origin,
		c.connectedAt.Format(time.RFC3339),
		len(c.subscriptions),
	)
}

func (c *Client) GetSubscriptions() map[string][]relay.Filter {
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
	case <-c.sendCh: // Check if already closed
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
			log.Println("[INFO] Waiting for full JSON...")
			continue
		}

		client.messageBuffer.Reset()

		var message []interface{}
		err = json.Unmarshal([]byte(fullMessage), &message)
		if err != nil {
			log.Printf("[ERROR] JSON parse error: %v", err)
			continue
		}

		messageType := message[0].(string)

		switch messageType {
		case "REQ":
			handlers.HandleReq(client, message)
		case "CLOSE":
			handlers.HandleClose(client, message)
		case "AUTH":
			if config.GetConfig().Auth.Enabled {
				handlers.HandleAuth(client, message)
			} else {
				log.Println("[WARN] Received AUTH message, but AUTH is disabled")
			}
		case "EVENT":
			handlers.HandleEvent(client, message)
		default:
			log.Printf("[WARN] Unknown message type: %s", messageType)
		}
	}
}

func clientWriter(client *Client) {
	ws := client.ws
	for msg := range client.sendCh {
		if err := websocket.Message.Send(ws, msg); err != nil {
			log.Printf("[ERROR] Failed to send message: %v | %s", err, client.ClientInfo())
			return // Don't call CloseClient() here, let WebSocketHandler handle it
		}
	}
}

func handleReadError(err error, ws *websocket.Conn) {
	clientsMu.Lock()
	client, exists := clients[ws]
	clientsMu.Unlock()

	clientInfo := "Unknown Client"
	if exists {
		clientInfo = client.ClientInfo()
	}

	if errors.Is(err, io.EOF) {
		log.Printf("[INFO] Client disconnected: %s", clientInfo)
	} else {
		log.Printf("[ERROR] WebSocket read error: %v | %s", err, clientInfo)
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
