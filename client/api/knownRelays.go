package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
)

// KnownRelaysHandler lists every relay the client is aware of (the "known" set:
// config seeds + pooled + directory-resolved) with its live pool status, for the
// relay manager's known-relays browser. Per-relay NIP-11 detail is fetched
// lazily via /api/v1/relay-info. Session-gated.
//
// @Summary      List known relays
// @Description  Every relay the client is aware of, with live pool status (connected/pinned/leased).
// @Tags         client
// @Produce      json
// @Success      200  {array}   core.KnownRelayStatus
// @Failure      401  {string}  string  "Authentication required"
// @Router       /api/v1/client/known-relays [get]
func KnownRelaysHandler(w http.ResponseWriter, r *http.Request) {
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
	if err := json.NewEncoder(w).Encode(cc.KnownRelaysWithStatus()); err != nil {
		log.ClientAPI().Error("Failed to encode known relays", "error", err)
	}
}

// RelayInfoHandler returns a relay's NIP-11 document (TTL-cached) for the
// known-relays browser's per-relay detail. Query: ?url=<relay url>. Returns an
// empty object when the relay advertises no NIP-11. Session-gated.
//
// @Summary      Get a relay's NIP-11 info
// @Description  A relay's NIP-11 document (name, software, supported NIPs, auth/payment flags), cached.
// @Tags         client
// @Produce      json
// @Param        url  query     string  true  "Relay URL"
// @Success      200  {object}  core.RelayInfo
// @Failure      401  {string}  string  "Authentication required"
// @Router       /api/v1/relay-info [get]
func RelayInfoHandler(w http.ResponseWriter, r *http.Request) {
	if session.SessionMgr.GetCurrentUser(r) == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "url parameter required", http.StatusBadRequest)
		return
	}
	cc := connection.GetCoreClient()
	if cc == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	info := cc.FetchRelayInfo(url)
	if info == nil {
		_, _ = w.Write([]byte("{}")) // no NIP-11 advertised — empty, not an error
		return
	}
	if err := json.NewEncoder(w).Encode(info); err != nil {
		log.ClientAPI().Error("Failed to encode relay info", "error", err)
	}
}
