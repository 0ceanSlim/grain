// Owner-gated proxy that lets the admin dashboard preview the keys
// at a domain's .well-known/nostr.json without giving the browser
// direct cross-origin fetch access. Used by the whitelist section
// to show "this domain admits N keys: name1 hex1, name2 hex2…"
// after an operator adds a domain.
package client

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// HandleAdminDomainKeys returns the parsed names map for a single
// domain's .well-known/nostr.json. Owner-gated via the cookie
// session — same gate /admin uses — so an unauthenticated visitor
// can't use grain as a SSRF proxy.
func HandleAdminDomainKeys(w http.ResponseWriter, r *http.Request) {
	user := session.SessionMgr.GetCurrentUser(r)
	owner := utils.GetRelayOwnerPubkey()
	if user == nil || utils.IsRelayUnowned() || !strings.EqualFold(user.PublicKey, owner) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	if domain == "" || strings.ContainsAny(domain, "/?#") {
		http.Error(w, "domain must be set and contain no path / query", http.StatusBadRequest)
		return
	}

	names, err := utils.FetchDomainNames(domain)
	if err != nil {
		log.ClientAPI().Info("Domain keys fetch failed",
			"domain", domain, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":  err.Error(),
			"domain": domain,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"domain": domain,
		"names":  names,
	})
}
