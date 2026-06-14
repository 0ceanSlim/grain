package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/client/stream"
	"github.com/0ceanslim/grain/server/utils/log"
)

// StreamHandler is the per-session SSE channel grain pushes live updates to (the
// login-hydration live-sync #87 and the streaming fetch path #77). The browser
// connects once via EventSource; grain streams JSON messages — one per `data:`
// line — until the client disconnects. A heartbeat comment keeps the connection
// (and any intermediary) alive.
//
// @Summary      Live update stream (SSE)
// @Description  Per-session Server-Sent Events channel for live updates. Connect with EventSource.
// @Tags         client
// @Produce      text/event-stream
// @Success      200  {string}  string  "SSE stream"
// @Failure      401  {string}  string  "Authentication required"
// @Router       /api/v1/stream [get]
func StreamHandler(w http.ResponseWriter, r *http.Request) {
	sess := session.SessionMgr.GetCurrentUser(r)
	if sess == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Clear the server's write deadline for this connection. The shared
	// http.Server sets WriteTimeout (a few seconds) to bound normal request
	// handlers, but that deadline caps the ENTIRE response — for a long-lived
	// SSE stream it kills the connection mid-flight (heartbeats don't help; the
	// deadline is absolute, not idle-based), so the browser reconnects in a
	// tight loop and live-sync thrashes. Streaming handlers must opt out.
	if rc := http.NewResponseController(w); rc != nil {
		if err := rc.SetWriteDeadline(time.Time{}); err != nil {
			log.ClientAPI().Debug("SSE: could not clear write deadline", "error", err)
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // tell any proxy not to buffer

	ch, unregister := stream.Default().Register(sess.PublicKey)
	defer unregister()

	// Opening comment so EventSource fires `onopen` right away.
	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()
	enc := json.NewEncoder(w)

	log.ClientAPI().Debug("SSE stream opened", "pubkey", sess.PublicKey)

	for {
		select {
		case <-r.Context().Done():
			log.ClientAPI().Debug("SSE stream closed", "pubkey", sess.PublicKey)
			return
		case <-heartbeat.C:
			_, _ = w.Write([]byte(": ping\n\n"))
			flusher.Flush()
		case msg, open := <-ch:
			if !open {
				return
			}
			_, _ = w.Write([]byte("data: "))
			_ = enc.Encode(msg) // JSON + trailing newline
			_, _ = w.Write([]byte("\n"))
			flusher.Flush()
		}
	}
}
