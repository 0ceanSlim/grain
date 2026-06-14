package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
)

// RelayListBuildRequest carries the desired relay list for one kind.
type RelayListBuildRequest struct {
	Kind    int                   `json:"kind"`    // 10002 | 10050 | 10006 | 10007 | 10012
	Entries []core.RelayListEntry `json:"entries"` // ordered; Read/Write apply to 10002 only
}

// BuildRelayListHandler assembles an UNSIGNED relay-list event for the session
// user, preserving any non-relay tags on their existing list, and returns it for
// the browser to sign. Publish the signed event via /api/v1/events/publish.
//
// @Summary      Build a relay-list event
// @Description  Assemble an unsigned NIP-65 (10002) / NIP-17 (10050) / NIP-51 (10006/10007/10012) relay-list event for the session user, preserving non-relay tags, and return it to sign client-side.
// @Tags         client
// @Accept       json
// @Produce      json
// @Success      200  {object}  nostr.Event
// @Failure      400  {string}  string  "Invalid request"
// @Failure      401  {string}  string  "Authentication required"
// @Router       /api/v1/user/relay-list/build [post]
func BuildRelayListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := session.SessionMgr.GetCurrentUser(r)
	if sess == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req RelayListBuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	existing := coreClient.FetchRelayList(sess.PublicKey, req.Kind)
	unsigned, err := core.AssembleRelayListEvent(existing, req.Kind, sess.PublicKey, req.Entries)
	if err != nil {
		log.ClientAPI().Warn("Relay-list build failed", "pubkey", sess.PublicKey, "kind", req.Kind, "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(unsigned); err != nil {
		log.ClientAPI().Error("Failed to encode built relay-list event", "error", err)
	}
}

// UserRelayListsResponse is a user's resolved relay lists across the kinds the
// relay manager edits.
type UserRelayListsResponse struct {
	Pubkey    string                `json:"pubkey"`
	NIP65     []core.RelayListEntry `json:"nip65"`     // 10002, with read/write flags
	DM        []string              `json:"dm"`        // 10050
	Blocked   []string              `json:"blocked"`   // 10006
	Search    []string              `json:"search"`    // 10007
	Favorites []string              `json:"favorites"` // 10012
	Encrypted core.EncryptedFlags   `json:"encrypted"` // NIP-51 lists with private (encrypted) entries
}

// GetUserRelayListsHandler resolves a user's relay lists (NIP-65 10002, NIP-17
// 10050, NIP-51 10006/10007/10012) for the relay manager. Defaults to the
// session user when no pubkey is given.
//
// @Summary      Get a user's relay lists
// @Description  Resolve a user's NIP-65 / NIP-17 / NIP-51 relay lists for the relay manager. Defaults to the session user.
// @Tags         client
// @Produce      json
// @Param        pubkey  query     string  false  "Hex pubkey (defaults to the session user)"
// @Success      200     {object}  UserRelayListsResponse
// @Failure      401     {string}  string  "Authentication required or pubkey parameter needed"
// @Router       /api/v1/user/relay-lists [get]
func GetUserRelayListsHandler(w http.ResponseWriter, r *http.Request) {
	pubkey := r.URL.Query().Get("pubkey")
	if pubkey == "" {
		sess := session.SessionMgr.GetCurrentUser(r)
		if sess == nil {
			http.Error(w, "Authentication required or pubkey parameter needed", http.StatusUnauthorized)
			return
		}
		pubkey = sess.PublicKey
	}

	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	lists := coreClient.ResolveUserRelayLists(pubkey)
	resp := UserRelayListsResponse{
		Pubkey:    pubkey,
		NIP65:     lists.NIP65,
		DM:        lists.DM,
		Blocked:   lists.Blocked,
		Search:    lists.Search,
		Favorites: lists.Favorites,
		Encrypted: lists.Encrypted,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.ClientAPI().Error("Failed to encode relay lists response", "error", err)
	}
}

// FixedRelaysRequest sets or clears the fixed-relay override.
type FixedRelaysRequest struct {
	Enabled bool     `json:"enabled"`
	Read    []string `json:"read"`
	Write   []string `json:"write"`
}

// FixedRelaysResponse reports the override state after the change.
type FixedRelaysResponse struct {
	Enabled bool `json:"enabled"`
}

// FixedRelaysHandler enables or disables the fixed-relay override — the advanced
// opt-out that bypasses the outbox model and uses a fixed read/write set (the
// "proxy" / aggregator override). Off by default and discouraged.
//
// @Summary      Set the fixed-relay override
// @Description  Enable (with read/write sets) or disable the fixed-relay override that bypasses outbox routing. Session-gated.
// @Tags         client
// @Accept       json
// @Produce      json
// @Success      200  {object}  FixedRelaysResponse
// @Failure      401  {string}  string  "Authentication required"
// @Router       /api/v1/client/fixed-relays [post]
func FixedRelaysHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := session.SessionMgr.GetCurrentUser(r)
	if sess == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req FixedRelaysRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	coreClient := connection.GetCoreClient()
	if coreClient == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	if req.Enabled {
		coreClient.SetFixedRelays(req.Read, req.Write)
	} else {
		coreClient.ClearFixedRelays()
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(FixedRelaysResponse{Enabled: coreClient.FixedRelaysEnabled()}); err != nil {
		log.ClientAPI().Error("Failed to encode fixed-relays response", "error", err)
	}
}
