package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ClientStatusHandler returns the core client's relay-pool status — how many
// relays are tracked versus currently connected — for the dashboard's relay
// indicator.
//
// @Summary      Core client relay status
// @Description  Relay pool counts (total, connected, pinned, leased) plus the index relay seed list.
// @Tags         client
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      405  {string}  string  "Method not allowed"
// @Router       /api/v1/client/status [get]
func ClientStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := connection.GetCoreClientStatus()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.ClientAPI().Error("Failed to encode client status", "error", err)
	}
}
