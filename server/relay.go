package relay

import (
	"encoding/json"
	"errors"

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
	mu            sync.Mutex
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

	client := &Client{
		ws:            ws,
		sendCh:        make(chan string, 100),
		subscriptions: make(map[string][]relay.Filter),
		rateLimiter:   config.GetRateLimiter(),
	}

	clientsMu.Lock()
	clients[ws] = client
	clientsMu.Unlock()

	log.Printf("New connection from IP: %s", utils.GetClientIP(ws.Request()))

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

func (c *Client) GetSubscriptions() map[string][]relay.Filter {
	return c.subscriptions
}

func (c *Client) CloseClient() {
	c.ws.Close()
	close(c.sendCh)
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
			log.Println("[ERROR] Failed to send:", err)
			return
		}
	}
}

func isValidJSON(s string) bool {
	var js interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func handleReadError(err error, ws *websocket.Conn) {
	if errors.Is(err, io.EOF) {
		log.Println("[INFO] Client closed the connection gracefully.")
	} else {
		log.Printf("[ERROR] WebSocket error: %v", err)
	}
	ws.Close()
}

// sendErrorMessage sends a formatted error message to the client and closes the WebSocket.
//func sendErrorMessage(ws *websocket.Conn, errMsg string) {
//	errMessage := fmt.Sprintf(`{"error": "%s"}`, errMsg)
//	_ = websocket.Message.Send(ws, errMessage)
//	ws.Close()
//}
//
//// handleSubscription handles WebSocket subscriptions.
//func handleSubscription(ws *websocket.Conn, message []interface{}, rateLimiter *config.RateLimiter, subscriptions map[string][]relay.Filter) {
//	mu.Lock()
//	defer mu.Unlock()
//
//	if clientSubscriptions[ws] >= config.GetConfig().Server.MaxSubscriptionsPerClient {
//		sendErrorMessage(ws, "too many subscriptions")
//		return
//	}
//	clientSubscriptions[ws]++
//
//	if allowed, errMsg := rateLimiter.AllowReq(); !allowed {
//		sendErrorMessage(ws, errMsg)
//		return
//	}
//
//	handlers.HandleReq(ws, message, subscriptions)
//}
