// Package stream is grain's server->browser push channel. A per-pubkey hub fans
// live messages (login-hydration live-sync #87, streaming fetch #77) out to a
// user's open pages over SSE; connections register on connect and drop on
// disconnect. Lifecycle callbacks let a consumer start work (e.g. the own-event
// subscription) only while a page is actually open.
package stream

import "sync"

// Message is a JSON-serialisable push sent to a user's open pages.
type Message struct {
	Type string `json:"type"`           // "list-updated" | "profile-updated" | ...
	Kind int    `json:"kind,omitempty"` // event kind, when relevant
	Data any    `json:"data,omitempty"` // optional payload
}

// conn is one open SSE connection's buffered outbound channel.
type conn struct {
	ch chan Message
}

// Hub fans push messages out to a user's open browser pages, keyed by pubkey.
type Hub struct {
	mu      sync.Mutex
	clients map[string]map[*conn]struct{} // pubkey -> set of connections
	onFirst func(pubkey string)           // first connection for a pubkey opened
	onLast  func(pubkey string)           // last connection for a pubkey closed
}

var hub = &Hub{clients: make(map[string]map[*conn]struct{})}

// Default returns the process-wide hub.
func Default() *Hub { return hub }

// SetLifecycle wires callbacks fired when a pubkey gains its first / loses its
// last open connection — so the live-sync subscription runs only while a page is
// open. Either may be nil.
func (h *Hub) SetLifecycle(onFirst, onLast func(string)) {
	h.mu.Lock()
	h.onFirst, h.onLast = onFirst, onLast
	h.mu.Unlock()
}

// Register adds a connection for pubkey and returns its message channel plus an
// unregister func to call on disconnect.
func (h *Hub) Register(pubkey string) (<-chan Message, func()) {
	c := &conn{ch: make(chan Message, 16)}

	h.mu.Lock()
	set, ok := h.clients[pubkey]
	if !ok {
		set = make(map[*conn]struct{})
		h.clients[pubkey] = set
	}
	first := len(set) == 0
	set[c] = struct{}{}
	onFirst := h.onFirst
	h.mu.Unlock()

	if first && onFirst != nil {
		onFirst(pubkey)
	}
	return c.ch, func() { h.unregister(pubkey, c) }
}

func (h *Hub) unregister(pubkey string, c *conn) {
	h.mu.Lock()
	set := h.clients[pubkey]
	last := false
	if set != nil {
		if _, ok := set[c]; ok {
			delete(set, c)
			close(c.ch)
		}
		if len(set) == 0 {
			delete(h.clients, pubkey)
			last = true
		}
	}
	onLast := h.onLast
	h.mu.Unlock()

	if last && onLast != nil {
		onLast(pubkey)
	}
}

// Push delivers a message to all of pubkey's open connections. Non-blocking: a
// connection whose buffer is full drops the message rather than stalling others.
func (h *Hub) Push(pubkey string, msg Message) {
	h.mu.Lock()
	set := h.clients[pubkey]
	conns := make([]*conn, 0, len(set))
	for c := range set {
		conns = append(conns, c)
	}
	h.mu.Unlock()

	for _, c := range conns {
		select {
		case c.ch <- msg:
		default:
		}
	}
}

// HasClients reports whether pubkey currently has any open connection.
func (h *Hub) HasClients(pubkey string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients[pubkey]) > 0
}
