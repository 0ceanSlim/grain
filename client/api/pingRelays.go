package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/server/utils/log"
)

// PingRelaysHandler measures TCP-connect latency for a set of relays, for the
// known-relays browser's "fastest first" sort. The browser sends only the rows
// currently in view (capped here too), so this never probes the whole known set.
// Session-gated. Results are short-TTL-cached server-side.
//
// @Summary      Ping relays
// @Description  TCP-connect latency (ms, -1 = unreachable) for a set of relays. Capped at 80 URLs per call.
// @Tags         client
// @Accept       json
// @Produce      json
// @Param        body  body      object{urls=[]string}  true  "Relay URLs to ping"
// @Success      200   {object}  map[string]int
// @Failure      401   {string}  string  "Authentication required"
// @Router       /api/v1/relays/ping [post]
func PingRelaysHandler(w http.ResponseWriter, r *http.Request) {
	if session.SessionMgr.GetCurrentUser(r) == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		URLs []string `json:"urls"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if len(req.URLs) == 0 {
		_, _ = w.Write([]byte("{}"))
		return
	}
	const maxURLs = 80
	if len(req.URLs) > maxURLs {
		req.URLs = req.URLs[:maxURLs]
	}
	cc := connection.GetCoreClient()
	if cc == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(cc.PingRelays(req.URLs)); err != nil {
		log.ClientAPI().Error("Failed to encode relay pings", "error", err)
	}
}
