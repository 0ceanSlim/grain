package helpers

import (
	"encoding/json"
	"net"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
	"golang.org/x/net/websocket"
)

// DialWithTimeout connects to a WebSocket with a timeout
func DialWithTimeout(url string, timeout time.Duration) (*websocket.Conn, error) {
	origin := "http://localhost/"
	
	// Create connection with timeout
	config, err := websocket.NewConfig(url, origin)
	if err != nil {
		return nil, err
	}
	
	// Set timeout on the underlying connection
	dialer := &net.Dialer{Timeout: timeout}
	config.Dialer = dialer
	
	return websocket.DialConfig(config)
}

// ReadMessageWithTimeout reads a WebSocket message with timeout
func ReadMessageWithTimeout(conn *websocket.Conn, timeout time.Duration) ([]byte, error) {
	conn.SetReadDeadline(time.Now().Add(timeout))
	
	message := make([]byte, 4096)
	n, err := conn.Read(message)
	if err != nil {
		return nil, err
	}
	
	return message[:n], nil
}

// SendCloseMessage sends a CLOSE message and waits for acknowledgment
func SendCloseMessage(conn *websocket.Conn, subscriptionID string) error {
	closeRequest := []interface{}{"CLOSE", subscriptionID}
	closeJSON, err := json.Marshal(closeRequest)
	if err != nil {
		return err
	}

	if _, err := conn.Write(closeJSON); err != nil {
		return err
	}

	// Wait for "CLOSED" response with timeout
	closedChan := make(chan struct{})
	go func() {
		for {
			message := make([]byte, 4096)
			n, err := conn.Read(message)
			if err != nil {
				break
			}

			var resp []interface{}
			if err := json.Unmarshal(message[:n], &resp); err != nil {
				break
			}

			if len(resp) > 1 && resp[0] == "CLOSED" && resp[1] == subscriptionID {
				closedChan <- struct{}{}
				return
			}
		}
	}()

	select {
	case <-closedChan:
		return nil
	case <-time.After(1 * time.Second):
		// No response, force close
		return nil
	}
}

// Legacy helper functions for compatibility during migration

// LegacyFetchFromRelay provides backward compatibility for direct WebSocket fetching
// This should be deprecated once all code uses the core client
func LegacyFetchFromRelay(relayURL, pubkey string, kind int) (interface{}, error) {
	log.Util().Warn("Using legacy fetch method - consider migrating to core client", 
		"relay", relayURL, "pubkey", pubkey, "kind", kind)
	
	conn, err := DialWithTimeout(relayURL, 10*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	subscriptionID := "legacy-sub"
	
	filter := map[string]interface{}{
		"authors": []string{pubkey},
		"kinds":   []int{kind},
		"limit":   1,
	}

	subRequest := []interface{}{"REQ", subscriptionID, filter}
	requestJSON, err := json.Marshal(subRequest)
	if err != nil {
		return nil, err
	}

	if err := websocket.Message.Send(conn, string(requestJSON)); err != nil {
		return nil, err
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	for {
		var messageStr string
		if err := websocket.Message.Receive(conn, &messageStr); err != nil {
			return nil, err
		}

		var response []interface{}
		if err := json.Unmarshal([]byte(messageStr), &response); err != nil {
			continue
		}

		switch response[0] {
		case "EVENT":
			if len(response) >= 3 {
				SendCloseMessage(conn, subscriptionID)
				return response[2], nil
			}
		case "EOSE":
			SendCloseMessage(conn, subscriptionID)
			return nil, nil
		case "NOTICE":
			if len(response) > 1 {
				log.Util().Warn("Relay notice", "relay", relayURL, "notice", response[1])
			}
		}
	}
}