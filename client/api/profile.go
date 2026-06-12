package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/session"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ProfileBuildRequest carries the user's existing kind-0 event and the field
// edits to apply.
type ProfileBuildRequest struct {
	Event *nostr.Event      `json:"event"` // existing kind-0 (may be null for a first profile)
	Edits map[string]string `json:"edits"`
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
// @Success      200  {object}  nostr.Event
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
