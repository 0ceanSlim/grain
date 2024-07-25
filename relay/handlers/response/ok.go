package response

import (
	"encoding/json"

	"golang.org/x/net/websocket"
)

func SendOK(ws *websocket.Conn, eventID string, status bool, message string) {
	response := []interface{}{"OK", eventID, status, message}
	responseBytes, _ := json.Marshal(response)
	websocket.Message.Send(ws, string(responseBytes))
}