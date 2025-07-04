package relay

import "golang.org/x/net/websocket"

// ClientInterface abstracts WebSocket clients
type ClientInterface interface {
	SendMessage(msg interface{})
	GetWS() *websocket.Conn
	GetSubscriptions() map[string][]Filter
	CloseClient()
	IsConnected() bool
}
