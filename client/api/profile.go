package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/session"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ProfileBuildRequest carries the user's existing kind-0 event and the field
// edits to apply.
type ProfileBuildRequest struct {
	Event     *nostr.Event      `json:"event"` // existing kind-0 (may be null for a first profile)
	Edits     map[string]string `json:"edits"`
	ClientTag *bool             `json:"client_tag,omitempty"` // nil = config default; else per-user override (#99)
}

// BuildProfileHandler merges the supplied field edits over the user's existing
// kind-0 metadata and returns the UNSIGNED result for the browser to sign. The
// merge preserves every existing content field and tag — only edited fields are
// changed, and each is dual-written to content and a tag (see
// core.AssembleProfileEvent).
//
// @Summary      Build an updated profile event
// @Description  Merge field edits over an existing kind-0 (preserving all other fields/tags) and return the unsigned event to sign client-side.
// @Tags         client
// @Accept       json
// @Produce      json
// @Success      200  {object}  relay.Event
// @Failure      400  {string}  string  "Invalid request"
// @Failure      401  {string}  string  "Authentication required"
// @Failure      403  {string}  string  "Pubkey mismatch"
// @Router       /api/v1/user/profile/build [post]
func BuildProfileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := session.SessionMgr.GetCurrentUser(r)
	if sess == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req ProfileBuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Edits) == 0 {
		http.Error(w, "No edits provided", http.StatusBadRequest)
		return
	}

	existing := req.Event
	if existing == nil {
		existing = &nostr.Event{}
	}
	// You can only build your OWN profile.
	if existing.PubKey != "" && existing.PubKey != sess.PublicKey {
		http.Error(w, "Existing event pubkey does not match the session", http.StatusForbidden)
		return
	}
	existing.PubKey = sess.PublicKey

	unsigned, err := core.AssembleProfileEvent(existing, req.Edits)
	if err != nil {
		log.ClientAPI().Warn("Profile build failed", "pubkey", sess.PublicKey, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	en, nm := resolveClientTag(req.ClientTag)
	core.ApplyClientTag(unsigned, en, nm)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(unsigned); err != nil {
		log.ClientAPI().Error("Failed to encode built profile event", "error", err)
	}
}

// PublishSignedRequest carries a fully-signed event to broadcast.
type PublishSignedRequest struct {
	Event *nostr.Event `json:"event"`
}

// PublishSignedResponse reports where a signed event was published.
type PublishSignedResponse struct {
	Success bool                   `json:"success"`
	EventID string                 `json:"eventId,omitempty"`
	Relays  []string               `json:"relays"`
	Results []core.BroadcastResult `json:"results"`
	Error   string                 `json:"error,omitempty"`
}

// PublishSignedHandler broadcasts a client-signed event via the outbox model.
// Unlike /api/v1/publish (which signs server-side from a private key), this
// accepts an already-signed event — the dashboard signs with the user's
// NIP-07 / NIP-46 / Amber signer in the browser.
//
// @Summary      Publish a pre-signed event
// @Description  Verify a client-signed event belongs to the session user, then broadcast it to the outbox-routed relays.
// @Tags         client-events
// @Accept       json
// @Produce      json
// @Success      200  {object}  PublishSignedResponse
// @Failure      400  {object}  PublishSignedResponse
// @Failure      401  {string}  string  "Authentication required"
// @Failure      403  {object}  PublishSignedResponse
// @Router       /api/v1/events/publish [post]
func PublishSignedHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := session.SessionMgr.GetCurrentUser(r)
	if sess == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req PublishSignedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Event == nil {
		writeSignedResponse(w, http.StatusBadRequest, PublishSignedResponse{Error: "Invalid request body"})
		return
	}
	if req.Event.PubKey != sess.PublicKey {
		writeSignedResponse(w, http.StatusForbidden, PublishSignedResponse{Error: "Event pubkey does not match the session"})
		return
	}
	if !core.VerifyEventSignature(req.Event) {
		writeSignedResponse(w, http.StatusBadRequest, PublishSignedResponse{Error: "Invalid event signature"})
		return
	}

	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		writeSignedResponse(w, http.StatusInternalServerError, PublishSignedResponse{Error: "Client not available"})
		return
	}

	relays := coreClient.RoutePublish(req.Event)
	results, err := coreClient.PublishEvent(req.Event, relays)
	if err != nil {
		writeSignedResponse(w, http.StatusInternalServerError, PublishSignedResponse{Error: err.Error(), Relays: relays})
		return
	}

	// A republished media-server or relay list must show immediately — drop the
	// cached resolution so the next lookup re-fetches it.
	if req.Event.Kind == 10063 || req.Event.Kind == 10096 {
		coreClient.InvalidateMediaServers(req.Event.PubKey)
	}
	if req.Event.Kind == 10002 || req.Event.Kind == 10050 {
		coreClient.InvalidateUserRelays(req.Event.PubKey)
	}
	switch req.Event.Kind {
	case 10002, 10050, 10006, 10007, 10012:
		coreClient.InvalidateUserRelayLists(req.Event.PubKey)
	}

	log.ClientAPI().Info("Published signed event", "kind", req.Event.Kind, "event_id", req.Event.ID, "relay_count", len(relays))
	writeSignedResponse(w, http.StatusOK, PublishSignedResponse{
		Success: true,
		EventID: req.Event.ID,
		Relays:  relays,
		Results: results,
	})
}

func writeSignedResponse(w http.ResponseWriter, status int, resp PublishSignedResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.ClientAPI().Error("Failed to encode publish response", "error", err)
	}
}

// publishStreamMsg is one NDJSON line of a streaming publish: a "start" line
// carrying the target relay list, a "result" line per relay as it resolves, and
// a final "done" line.
type publishStreamMsg struct {
	Type     string   `json:"type"`             // "start" | "result" | "done"
	Relays   []string `json:"relays,omitempty"` // start: the target relays
	RelayURL string   `json:"relayURL,omitempty"`
	Sent     bool     `json:"sent"`     // the EVENT was delivered
	Accepted bool     `json:"accepted"` // the relay's NIP-20 OK verdict
	Reason   string   `json:"reason,omitempty"`
	Message  string   `json:"message,omitempty"` // send-failure detail
}

// PublishSignedStreamHandler is the streaming counterpart of
// PublishSignedHandler: it broadcasts a client-signed event and streams each
// relay's result as NDJSON (one JSON object per line) the moment it resolves, so
// the browser can show a live, count-up broadcast toast.
//
// @Summary      Publish a pre-signed event (streaming)
// @Description  Verify a client-signed event belongs to the session user, broadcast it via the outbox model, and stream per-relay results as newline-delimited JSON.
// @Tags         client-events
// @Accept       json
// @Produce      application/x-ndjson
// @Success      200  {string}  string  "NDJSON stream of start/result/done lines"
// @Failure      401  {string}  string  "Authentication required"
// @Failure      403  {string}  string  "Pubkey mismatch"
// @Router       /api/v1/events/publish/stream [post]
func PublishSignedStreamHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := session.SessionMgr.GetCurrentUser(r)
	if sess == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req PublishSignedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Event == nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Event.PubKey != sess.PublicKey {
		http.Error(w, "Event pubkey does not match the session", http.StatusForbidden)
		return
	}
	if !core.VerifyEventSignature(req.Event) {
		http.Error(w, "Invalid event signature", http.StatusBadRequest)
		return
	}

	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Opt out of the server's WriteTimeout: a broadcast to slow relays can run
	// longer than the (few-second) deadline, which would otherwise cut the
	// NDJSON stream and truncate the live toast's per-relay results. Same fix
	// as StreamHandler.
	if rc := http.NewResponseController(w); rc != nil {
		if err := rc.SetWriteDeadline(time.Time{}); err != nil {
			log.ClientAPI().Debug("publish stream: could not clear write deadline", "error", err)
		}
	}

	relays := coreClient.RoutePublish(req.Event)

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no") // tell any proxy not to buffer the stream
	enc := json.NewEncoder(w)

	_ = enc.Encode(publishStreamMsg{Type: "start", Relays: relays})
	flusher.Flush()

	for res := range coreClient.PublishEventStream(req.Event, relays) {
		msg := publishStreamMsg{
			Type:     "result",
			RelayURL: res.RelayURL,
			Sent:     res.Success,
			Accepted: res.Accepted,
			Reason:   res.Reason,
		}
		if !res.Success {
			msg.Message = res.Message
		}
		_ = enc.Encode(msg)
		flusher.Flush()
	}

	_ = enc.Encode(publishStreamMsg{Type: "done"})
	flusher.Flush()

	// Same cache invalidation as the non-streaming handler.
	if req.Event.Kind == 10063 || req.Event.Kind == 10096 {
		coreClient.InvalidateMediaServers(req.Event.PubKey)
	}
	if req.Event.Kind == 10002 || req.Event.Kind == 10050 {
		coreClient.InvalidateUserRelays(req.Event.PubKey)
	}
	switch req.Event.Kind {
	case 10002, 10050, 10006, 10007, 10012:
		coreClient.InvalidateUserRelayLists(req.Event.PubKey)
	}
	log.ClientAPI().Info("Streamed signed event publish", "kind", req.Event.Kind, "event_id", req.Event.ID, "relay_count", len(relays))
}
