package relay

import "golang.org/x/net/websocket"

// ClientInterface abstracts WebSocket clients
type ClientInterface interface {
	SendMessage(msg interface{})
	GetWS() *websocket.Conn
	GetSubscriptions() map[string][]Filter
	SetSubscription(subID string, filters []Filter)
	DeleteSubscription(subID string)
	SubscriptionCount() int
	ForEachSubscription(fn func(subID string, filters []Filter))
	CloseClient()
	IsConnected() bool
}
