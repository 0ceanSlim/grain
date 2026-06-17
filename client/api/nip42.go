package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/session"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// AuthRequestsHandler lists the relays that have issued a NIP-42 AUTH challenge
// this session, each with whether we've already authed. Session-gated.
//
// @Summary      List NIP-42 AUTH requests
// @Description  Relays that have challenged the client for AUTH, with session authed status.
// @Tags         client
// @Produce      json
// @Success      200  {array}   core.AuthState
// @Failure      401  {string}  string  "Authentication required"
// @Router       /api/v1/client/auth-requests [get]
func AuthRequestsHandler(w http.ResponseWriter, r *http.Request) {
	if session.SessionMgr.GetCurrentUser(r) == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	cc := connection.GetCoreClient()
	if cc == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cc.AuthRequests()); err != nil {
		log.ClientAPI().Error("Failed to encode auth requests", "error", err)
	}
}

// SubmitAuthHandler relays a browser-signed kind-22242 event to a relay,
// answering its NIP-42 challenge. The browser builds + signs the event (it holds
// the key); grain only forwards it on the challenged connection. Session-gated.
//
// @Summary      Answer a NIP-42 AUTH challenge
// @Description  Relay a browser-signed kind-22242 auth event to a relay.
// @Tags         client
// @Accept       json
// @Produce      json
// @Param        body  body      object{relay=string,event=object}  true  "Relay URL + signed kind-22242 event"
// @Success      200   {object}  map[string]any
// @Failure      400   {string}  string  "Invalid request"
// @Failure      401   {string}  string  "Authentication required"
// @Router       /api/v1/client/auth [post]
func SubmitAuthHandler(w http.ResponseWriter, r *http.Request) {
	if session.SessionMgr.GetCurrentUser(r) == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Relay string       `json:"relay"`
		Event *nostr.Event `json:"event"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	if req.Relay == "" || req.Event == nil {
		http.Error(w, "relay and event are required", http.StatusBadRequest)
		return
	}
	if req.Event.Kind != 22242 {
		http.Error(w, "event must be kind 22242 (NIP-42)", http.StatusBadRequest)
		return
	}
	cc := connection.GetCoreClient()
	if cc == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}
	if err := cc.SendAuth(req.Relay, req.Event); err != nil {
		log.ClientAPI().Warn("Failed to send AUTH", "relay", req.Relay, "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "error": err.Error()})
		return
	}
	log.ClientAPI().Info("Sent NIP-42 AUTH", "relay", req.Relay)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// RemoveAuthHandler forgets a relay's session AUTH state (revoke trust).
// Session-gated.
//
// @Summary      Revoke a relay's session AUTH
// @Tags         client
// @Accept       json
// @Produce      json
// @Param        body  body      object{relay=string}  true  "Relay URL"
// @Success      200   {object}  map[string]any
// @Failure      401   {string}  string  "Authentication required"
// @Router       /api/v1/client/auth/remove [post]
func RemoveAuthHandler(w http.ResponseWriter, r *http.Request) {
	if session.SessionMgr.GetCurrentUser(r) == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Relay string `json:"relay"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Relay == "" {
		http.Error(w, "relay is required", http.StatusBadRequest)
		return
	}
	cc := connection.GetCoreClient()
	if cc == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}
	cc.RemoveAuth(req.Relay)
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// writeJSON writes v as a JSON response with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.ClientAPI().Error("Failed to encode JSON response", "error", err)
	}
}
