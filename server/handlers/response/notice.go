package response

import (
	"encoding/json"

	"golang.org/x/net/websocket"
)

func SendNotice(ws *websocket.Conn, pubKey, message string) {
	notice := []interface{}{"NOTICE", pubKey, message}
	noticeBytes, _ := json.Marshal(notice)
	websocket.Message.Send(ws, string(noticeBytes))
}
