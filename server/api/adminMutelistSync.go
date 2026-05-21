package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/config"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// fetchAdminMutelistResponse carries the owner's raw mute list events to the
// browser for client-side decryption. Encrypted `.content` is passed through
// untouched — the relay can't read it, the owner's signer can.
type fetchAdminMutelistResponse struct {
	Events []*nostr.Event `json:"events"`
}

// FetchAdminMutelist returns the relay owner's latest kind:10000 +
// kind:30000(d:"mute") events so the browser can decrypt their private
// `.content` (#60). The relay does the relay-fetching (outbox resolution +
// subscription) it already does for public mutelist authors; only decryption
// has to happen browser-side, where the owner's key lives. GET, owner-gated.
//
// @Summary      Fetch owner's mute list events
// @Description  Returns the relay owner's raw NIP-51 mute list events (incl. encrypted content) for browser-side decryption. Owner-only via NIP-98.
// @Tags         relay-admin
// @Produce      json
// @Success      200  {object}  fetchAdminMutelistResponse
// @Failure      401  {string}  string  "Unauthorized"
// @Failure      403  {string}  string  "Forbidden: signer is not relay owner"
// @Router       /api/v1/relay/admin/mutelist/fetch [get]
func FetchAdminMutelist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	owner, ok := RequireOwner(w, r)
	if !ok {
		return
	}

	events, err := config.FetchAuthorMuteListEvents(owner)
	if err != nil {
		log.RelayAPI().Warn("Admin mutelist fetch failed",
			"owner", owner, "error", err)
		http.Error(w, "Failed to fetch mute list events: "+err.Error(), http.StatusBadGateway)
		return
	}

	log.RelayAPI().Info("Admin mutelist fetch", "owner", owner, "events", len(events))

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(fetchAdminMutelistResponse{Events: events}); err != nil {
		log.RelayAPI().Error("Admin mutelist fetch: encode response failed", "error", err)
	}
}

// syncAdminMutelistRequest is the POST body: the decrypted pubkey set the
// owner's browser extracted from their NIP-51 mute lists (kind:10000 +
// kind:30000 d:"mute", public tags ∪ NIP-44/NIP-04-decrypted content).
type syncAdminMutelistRequest struct {
	Pubkeys []string `json:"pubkeys"`
}

// syncAdminMutelistResponse echoes what was stored so the dashboard can show
// the last-synced timestamp and contributed count without a second round trip.
type syncAdminMutelistResponse struct {
	OK       bool   `json:"ok"`
	Count    int    `json:"count"`
	SyncedAt int64  `json:"synced_at"`
	Pubkey   string `json:"pubkey"`
}

// SyncAdminMutelist accepts the relay owner's decrypted private mute list and
// folds it into the blacklist (#60).
//
// The relay can't decrypt NIP-51 `.content` — it has no private key — so
// decryption happens in the owner's browser and only the resulting plain
// pubkey list reaches this endpoint. POST-only, gated by RequireOwner
// (NIP-98 + relay-owner check). An empty pubkey list clears the owner's
// contribution (un-sync).
//
// @Summary      Sync owner's private mute list
// @Description  Stores the relay owner's decrypted NIP-51 private mute pubkeys as a blacklist source. Owner-only via NIP-98.
// @Tags         relay-admin
// @Accept       json
// @Produce      json
// @Param        body  body      syncAdminMutelistRequest  true  "Decrypted pubkey set"
// @Success      200   {object}  syncAdminMutelistResponse
// @Failure      401   {string}  string  "Unauthorized"
// @Failure      403   {string}  string  "Forbidden: signer is not relay owner"
// @Router       /api/v1/relay/admin/mutelist/sync [post]
func SyncAdminMutelist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// NIP-98 + owner gate. RequireOwner writes the 401/403 response itself
	// on failure, so we just early-return.
	owner, ok := RequireOwner(w, r)
	if !ok {
		return
	}

	var req syncAdminMutelistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.RelayAPI().Warn("Admin mutelist sync: bad request body",
			"client_ip", utils.GetClientIP(r), "error", err)
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	meta, err := config.SetAdminMutelist(owner, req.Pubkeys)
	if err != nil {
		log.RelayAPI().Error("Admin mutelist sync: persist failed",
			"owner", owner, "error", err)
		http.Error(w, "Failed to persist mute list: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.RelayAPI().Info("Admin mutelist sync",
		"owner", owner, "received", len(req.Pubkeys), "stored", meta.Count)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(syncAdminMutelistResponse{
		OK:       true,
		Count:    meta.Count,
		SyncedAt: meta.SyncedAt,
		Pubkey:   owner,
	}); err != nil {
		log.RelayAPI().Error("Admin mutelist sync: encode response failed", "error", err)
	}
}
