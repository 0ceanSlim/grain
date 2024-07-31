package response

import (
	"encoding/json"

	"golang.org/x/net/websocket"
)

func SendClosed(ws *websocket.Conn, subID string, message string) {
	closeMsg := []interface{}{"CLOSED", subID, message}
	closeBytes, _ := json.Marshal(closeMsg)
	websocket.Message.Send(ws, string(closeBytes))
}
