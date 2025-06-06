package helpers

import (
	"encoding/json"
	"net"
	"time"

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