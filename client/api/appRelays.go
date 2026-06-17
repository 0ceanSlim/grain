package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
)

// AppRelaysPayload is the session's locally-configured ("app") relay roles —
// editable preferences seeded from the operator's config, not published Nostr
// lists. Indexer drives discovery; Broadcast mirrors writes; Local/Trusted are
// stored but inert until their wiring lands (Local routing; Trusted NIP-42 AUTH).
type AppRelaysPayload struct {
	Indexer   []string `json:"indexer"`
	Broadcast []string `json:"broadcast"`
	Local     []string `json:"local"`
	Trusted   []string `json:"trusted"`
}

// AppRelaysHandler gets (GET) or replaces (POST) the session's app-relay
// preferences. POST sets all four roles from the payload; an empty list clears
// that role (Indexer then falls back to the configured default). Session-gated.
//
// @Summary      Get or set app-relay preferences
// @Description  The session's locally-configured Indexer/Broadcast/Local/Trusted relays. GET returns them; POST replaces them.
// @Tags         client
// @Produce      json
// @Success      200  {object}  AppRelaysPayload
// @Failure      401  {string}  string  "Authentication required"
// @Router       /api/v1/client/app-relays [get]
func AppRelaysHandler(w http.ResponseWriter, r *http.Request) {
	sess := session.SessionMgr.GetCurrentUser(r)
	if sess == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	cc := connection.GetCoreClient()
	if cc == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	if r.Method == http.MethodPost {
		var req AppRelaysPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		cc.SetAppRelays(core.RoleIndexer, req.Indexer)
		cc.SetAppRelays(core.RoleBroadcast, req.Broadcast)
		cc.SetAppRelays(core.RoleLocal, req.Local)
		cc.SetAppRelays(core.RoleTrusted, req.Trusted)
		log.ClientAPI().Info("App relays updated", "pubkey", sess.PublicKey)
	}

	resp := AppRelaysPayload{
		Indexer:   cc.AppRelays(core.RoleIndexer),
		Broadcast: cc.AppRelays(core.RoleBroadcast),
		Local:     cc.AppRelays(core.RoleLocal),
		Trusted:   cc.AppRelays(core.RoleTrusted),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.ClientAPI().Error("Failed to encode app relays", "error", err)
	}
}
