package relay

import (
	"encoding/json"
	"fmt"
	"grain/config"
	"grain/server/handlers"
	relay "grain/server/types"
	"grain/server/utils"
	"log"
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

	var msg string
	rateLimiter := config.GetRateLimiter()

	subscriptions := make(map[string][]relay.Filter) // Subscription map scoped to the connection
	clientSubscriptions[ws] = 0

	for {
		err := websocket.Message.Receive(ws, &msg)
		if err != nil {
			fmt.Println("Error receiving message:", err)
			ws.Close()
			return
		}
		fmt.Println("Received message:", msg)

		if allowed, msg := rateLimiter.AllowWs(); !allowed {
			websocket.Message.Send(ws, fmt.Sprintf(`{"error": "%s"}`, msg))
			ws.Close()
			return
		}

		var message []interface{}
		err = json.Unmarshal([]byte(msg), &message)
		if err != nil {
			fmt.Println("Error parsing message:", err)
			continue
		}

		if len(message) < 2 {
			fmt.Println("Invalid message format")
			continue
		}

		messageType, ok := message[0].(string)
		if !ok {
			fmt.Println("Invalid message type")
			continue
		}

		switch messageType {
		case "EVENT":
			handlers.HandleEvent(ws, message)
		case "REQ":
			mu.Lock()
			if clientSubscriptions[ws] >= config.GetConfig().Server.MaxSubscriptionsPerClient {
				websocket.Message.Send(ws, `{"error": "too many subscriptions"}`)
				mu.Unlock()
				continue
			}
			clientSubscriptions[ws]++
			mu.Unlock()
			if allowed, msg := rateLimiter.AllowReq(); !allowed {
				websocket.Message.Send(ws, fmt.Sprintf(`{"error": "%s"}`, msg))
				ws.Close()
				return
			}
			handlers.HandleReq(ws, message, subscriptions)
		case "AUTH":
			if config.GetConfig().Auth.Enabled {
				handlers.HandleAuth(ws, message)
			} else {
				fmt.Println("Received AUTH message, but AUTH is disabled")
			}
		case "CLOSE":
			handlers.HandleClose(ws, message)
		default:
			fmt.Println("Unknown message type:", messageType)
		}
	}
}
