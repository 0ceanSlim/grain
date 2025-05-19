package handlers

import (
	"encoding/json"
	"log"
	"time"

	"github.com/0ceanslim/grain/client/types"

	"github.com/gorilla/websocket"
)

const WebSocketTimeout = 2 * time.Second // Set timeout duration

func FetchUserMetadata(publicKey string, relays []string) (*types.UserMetadata, error) {
	for _, url := range relays {
		log.Printf("Connecting to WebSocket: %s\n", url)
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			log.Printf("Failed to connect to WebSocket: %v\n", err)
			continue
		}
		defer conn.Close()

		filter := types.SubscriptionFilter{
			Authors: []string{publicKey},
			Kinds:   []int{0}, // Kind 0 corresponds to metadata (NIP-01)
		}

		subRequest := []interface{}{
			"REQ",
			"sub1",
			filter,
		}

		requestJSON, err := json.Marshal(subRequest)
		if err != nil {
			log.Printf("Failed to marshal subscription request: %v\n", err)
			return nil, err
		}

		log.Printf("Sending subscription request: %s\n", requestJSON)

		if err := conn.WriteMessage(websocket.TextMessage, requestJSON); err != nil {
			log.Printf("Failed to send subscription request: %v\n", err)
			return nil, err
		}

		// WebSocket message or timeout handling
		msgChan := make(chan []byte)
		errChan := make(chan error)

		go func() {
			_, message, err := conn.ReadMessage()
			if err != nil {
				errChan <- err
			} else {
				msgChan <- message
			}
		}()

		select {
		case message := <-msgChan:
			log.Printf("Received WebSocket message: %s\n", message)
			var response []interface{}
			if err := json.Unmarshal(message, &response); err != nil {
				log.Printf("Failed to unmarshal response: %v\n", err)
				continue
			}

			if response[0] == "EVENT" {
				eventData, err := json.Marshal(response[2])
				if err != nil {
					log.Printf("Failed to marshal event data: %v\n", err)
					continue
				}

				var event types.NostrEvent
				if err := json.Unmarshal(eventData, &event); err != nil {
					log.Printf("Failed to parse event data: %v\n", err)
					continue
				}

				log.Printf("Received Nostr event: %+v\n", event)

				var content types.UserMetadata
				if err := json.Unmarshal([]byte(event.Content), &content); err != nil {
					log.Printf("Failed to parse content JSON: %v\n", err)
					continue
				}
				return &content, nil
			} else if response[0] == "EOSE" {
				log.Println("End of subscription signal received")
				break
			}
		case err := <-errChan:
			log.Printf("Error reading WebSocket message: %v\n", err)
			continue
		case <-time.After(WebSocketTimeout):
			log.Printf("WebSocket response timeout from %s\n", url)
			continue
		}
	}
	return nil, nil
}
