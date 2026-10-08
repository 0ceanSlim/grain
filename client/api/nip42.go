package api

import (
	"encoding/json"
	"net/http"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// AuthRequestsHandler lists the relays asking the logged-in user for NIP-42
// AUTH, each with whether this user has authenticated to it. Session-gated;
// one user's AUTH state is never shown to, or shared with, another.
//
// @Summary      List NIP-42 AUTH requests
// @Description  Relays that have challenged for AUTH, with this user's authed status.
// @Tags         client
// @Produce      json
// @Success      200  {array}   core.AuthState
// @Failure      401  {string}  string  "Authentication required"
// @Router       /api/v1/client/auth-requests [get]
func AuthRequestsHandler(w http.ResponseWriter, r *http.Request) {
	uc := sessionUser(w, r)
	if uc == nil {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(uc.AuthStates()); err != nil {
		log.ClientAPI().Error("Failed to encode auth requests", "error", err)
	}
}

// AuthChallengeHandler opens the logged-in user's own connection to a relay
// and returns the NIP-42 challenge the relay issued on it, for the browser to
// sign. AUTH is per connection, so each user answers a challenge on a socket
// only their requests use. Session-gated.
//
// @Summary      Get a NIP-42 AUTH challenge for this user
// @Tags         client
// @Accept       json
// @Produce      json
// @Param        body  body      object{relay=string}  true  "Relay URL"
// @Success      200   {object}  map[string]any
// @Failure      400   {string}  string  "Invalid request"
// @Failure      401   {string}  string  "Authentication required"
// @Router       /api/v1/client/auth/challenge [post]
func AuthChallengeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	uc := sessionUser(w, r)
	if uc == nil {
		return
	}
	var req struct {
		Relay string `json:"relay"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Relay == "" {
		http.Error(w, "relay is required", http.StatusBadRequest)
		return
	}
	challenge, err := uc.AuthChallenge(r.Context(), req.Relay)
	if err != nil {
		log.ClientAPI().Debug("No AUTH challenge", "relay", req.Relay, "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "challenge": challenge})
}

// SubmitAuthHandler answers a relay's NIP-42 challenge for the logged-in user
// with a browser-signed kind-22242 event. The event must be signed by the
// session's pubkey and answer the challenge from AuthChallengeHandler; it is
// sent on the user's own connection. Session-gated.
//
// @Summary      Answer a NIP-42 AUTH challenge
// @Description  Send a browser-signed kind-22242 auth event on this user's connection.
// @Tags         client
// @Accept       json
// @Produce      json
// @Param        body  body      object{relay=string,event=object}  true  "Relay URL + signed kind-22242 event"
// @Success      200   {object}  map[string]any
// @Failure      400   {string}  string  "Invalid request"
// @Failure      401   {string}  string  "Authentication required"
// @Router       /api/v1/client/auth [post]
func SubmitAuthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	uc := sessionUser(w, r)
	if uc == nil {
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
	if err := uc.Authenticate(r.Context(), req.Relay, req.Event); err != nil {
		log.ClientAPI().Warn("NIP-42 AUTH failed", "relay", req.Relay, "pubkey", uc.PublicKey(), "error", err)
		writeJSON(w, http.StatusOK, map[string]any{"success": false, "error": err.Error()})
		return
	}
	log.ClientAPI().Info("Sent NIP-42 AUTH", "relay", req.Relay, "pubkey", uc.PublicKey())
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// RemoveAuthHandler ends the logged-in user's NIP-42 session with a relay by
// closing their own connection to it. Session-gated.
//
// @Summary      Revoke a relay's AUTH for this user
// @Tags         client
// @Accept       json
// @Produce      json
// @Param        body  body      object{relay=string}  true  "Relay URL"
// @Success      200   {object}  map[string]any
// @Failure      401   {string}  string  "Authentication required"
// @Router       /api/v1/client/auth/remove [post]
func RemoveAuthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	uc := sessionUser(w, r)
	if uc == nil {
		return
	}
	var req struct {
		Relay string `json:"relay"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Relay == "" {
		http.Error(w, "relay is required", http.StatusBadRequest)
		return
	}
	uc.Deauthenticate(req.Relay)
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
