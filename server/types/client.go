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
	// AllowReq checks the client's per-connection REQ rate limiter.
	// Returns (true, "") if allowed, (false, reason) if rate limited.
	AllowReq() (bool, string)
	// AllowEvent checks the client's per-connection event rate limiter.
	AllowEvent(kind int, category string) (bool, string)
}
