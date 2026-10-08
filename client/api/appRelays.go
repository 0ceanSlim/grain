package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/core"
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

// AppRelaysHandler gets (GET) or replaces (POST) the logged-in user's
// app-relay preferences. POST sets all four roles from the payload; an empty
// list clears that role's override (falling back to the configured default).
// The preferences are this user's alone. Session-gated.
//
// @Summary      Get or set app-relay preferences
// @Description  This user's locally-configured Indexer/Broadcast/Local/Trusted relays. GET returns them; POST replaces them.
// @Tags         client
// @Produce      json
// @Success      200  {object}  AppRelaysPayload
// @Failure      401  {string}  string  "Authentication required"
// @Router       /api/v1/client/app-relays [get]
func AppRelaysHandler(w http.ResponseWriter, r *http.Request) {
	uc := sessionUser(w, r)
	if uc == nil {
		return
	}

	if r.Method == http.MethodPost {
		var req AppRelaysPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		uc.SetAppRelays(core.RoleIndexer, req.Indexer)
		uc.SetAppRelays(core.RoleBroadcast, req.Broadcast)
		uc.SetAppRelays(core.RoleLocal, req.Local)
		uc.SetAppRelays(core.RoleTrusted, req.Trusted)
		log.ClientAPI().Info("App relays updated", "pubkey", uc.PublicKey())
	}

	resp := AppRelaysPayload{
		Indexer:   uc.AppRelays(core.RoleIndexer),
		Broadcast: uc.AppRelays(core.RoleBroadcast),
		Local:     uc.AppRelays(core.RoleLocal),
		Trusted:   uc.AppRelays(core.RoleTrusted),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.ClientAPI().Error("Failed to encode app relays", "error", err)
	}
}
