package relay

import (
	"encoding/json"
	"errors"
	"fmt"
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

// Global connection count
var (
	currentConnections int
	mu                 sync.Mutex
)

// Client subscription count
var clientSubscriptions = make(map[*websocket.Conn]int)

func WebSocketHandler(ws *websocket.Conn) {
	defer func() {
		mu.Lock()
		currentConnections--
		delete(clientSubscriptions, ws)
		mu.Unlock()
		ws.Close()
	}()

	mu.Lock()
	if currentConnections >= config.GetConfig().Server.MaxConnections {
		websocket.Message.Send(ws, `{"error": "too many connections"}`)
		mu.Unlock()
		return
	}
	currentConnections++
	mu.Unlock()

	clientInfo := utils.ClientInfo{
		IP:        utils.GetClientIP(ws.Request()),
		UserAgent: ws.Request().Header.Get("User-Agent"),
		Origin:    ws.Request().Header.Get("Origin"),
	}

	log.Printf("New connection from IP: %s, User-Agent: %s, Origin: %s", clientInfo.IP, clientInfo.UserAgent, clientInfo.Origin)

	var messageBuffer strings.Builder // Buffer to accumulate full JSON messages
	rateLimiter := config.GetRateLimiter()
	subscriptions := make(map[string][]relay.Filter)
	clientSubscriptions[ws] = 0

	for {
		var chunk string
		err := websocket.Message.Receive(ws, &chunk)
		if err != nil {
			handleReadError(err, ws)
			return
		}

		// ✅ Append received chunk to buffer
		messageBuffer.WriteString(chunk)

		// ✅ Check if the accumulated data is a valid JSON
		fullMessage := messageBuffer.String()
		if !isValidJSON(fullMessage) {
			log.Println("[INFO] Received fragmented message, waiting for more data...")
			continue // Wait for the next chunk
		}

		// ✅ Now we have a complete JSON message, process it
		msg := fullMessage
		messageBuffer.Reset() // Clear buffer for next message

		log.Printf("Received complete message: %s", msg)

		// ✅ Check rate limits
		if allowed, errMsg := rateLimiter.AllowWs(); !allowed {
			sendErrorMessage(ws, errMsg)
			return
		}

		var message []interface{}
		err = json.Unmarshal([]byte(msg), &message)
		if err != nil {
			log.Printf("[ERROR] Failed to parse message: %v", err)
			continue
		}

		if len(message) < 2 {
			log.Println("[WARN] Invalid message format")
			continue
		}

		messageType, ok := message[0].(string)
		if !ok {
			log.Println("[WARN] Invalid message type")
			continue
		}

		switch messageType {
		case "EVENT":
			handlers.HandleEvent(ws, message)
		case "REQ":
			handleSubscription(ws, message, rateLimiter, subscriptions)
		case "AUTH":
			if config.GetConfig().Auth.Enabled {
				handlers.HandleAuth(ws, message)
			} else {
				log.Println("[WARN] Received AUTH message, but AUTH is disabled")
			}
		case "CLOSE":
			handlers.HandleClose(ws, message)
		default:
			log.Printf("[WARN] Unknown message type: %s", messageType)
		}
	}
}

// ✅ Helper function to check if a string is valid JSON
func isValidJSON(s string) bool {
	var js interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

// handleReadError handles errors during message reception.
func handleReadError(err error, ws *websocket.Conn) {
	if errors.Is(err, io.EOF) {
		log.Println("[INFO] Client closed the connection gracefully.")
	} else if errors.Is(err, io.ErrUnexpectedEOF) {
		log.Println("[ERROR] Unexpected EOF during message read.")
	} else if errors.Is(err, io.ErrClosedPipe) {
		log.Println("[ERROR] Read/write attempted on a closed WebSocket pipe.")
	} else {
		log.Printf("[ERROR] Unexpected WebSocket error: %v", err)
	}
	ws.Close()
}

// sendErrorMessage sends a formatted error message to the client and closes the WebSocket.
func sendErrorMessage(ws *websocket.Conn, errMsg string) {
	errMessage := fmt.Sprintf(`{"error": "%s"}`, errMsg)
	_ = websocket.Message.Send(ws, errMessage)
	ws.Close()
}

// handleSubscription handles WebSocket subscriptions.
func handleSubscription(ws *websocket.Conn, message []interface{}, rateLimiter *config.RateLimiter, subscriptions map[string][]relay.Filter) {
	mu.Lock()
	defer mu.Unlock()

	if clientSubscriptions[ws] >= config.GetConfig().Server.MaxSubscriptionsPerClient {
		sendErrorMessage(ws, "too many subscriptions")
		return
	}
	clientSubscriptions[ws]++

	if allowed, errMsg := rateLimiter.AllowReq(); !allowed {
		sendErrorMessage(ws, errMsg)
		return
	}

	handlers.HandleReq(ws, message, subscriptions)
}
