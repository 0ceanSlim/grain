package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/server/db/nostrdb"
	"github.com/0ceanslim/grain/server/utils/log"
)

// GetRelayStats serves a compact, public snapshot of the relay's live vitals for
// the dashboard: storage fill, total events, active connections, uptime, and
// build version. Read-only and cheap — atomic counters plus a single ndb_stat,
// no scan. Client-pool / NIP-66 discovery counts come from the separate
// /api/v1/client/status; this is the relay-side half.
//
// @Summary      Relay vitals
// @Description  Storage fill, event count, connections, uptime, version — for the dashboard.
// @Tags         relay
// @Produce      json
// @Success      200  {object}  map[string]any
// @Router       /api/v1/relay/stats [get]
func GetRelayStats(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{
		"server": statsHook(), // connections, uptime, version (wired in startup)
	}

	if db := nostrdb.GetDB(); db != nil {
		if used, total, ok := db.MapUsage(); ok {
			var pct float64
			if total > 0 {
				pct = float64(used) / float64(total) * 100
			}
			out["storage"] = map[string]any{
				"used_bytes":  used,
				"total_bytes": total,
				"pct":         pct,
			}
		}
		ev := map[string]any{}
		if n, err := db.NoteCount(); err == nil {
			ev["total"] = n
		}
		if kinds, err := db.KindDistribution(); err == nil {
			ev["by_kind"] = kinds
		}
		if len(ev) > 0 {
			out["events"] = ev
		}
		// Writer-failure counter (rc2 durability): 0 = healthy, >0 = silent
		// write loss (map full / bad txn) — a real relay-health signal.
		out["write_errors"] = db.WriteErrorCount()
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(out); err != nil {
		log.RelayAPI().Error("Failed to encode relay stats", "error", err)
	}
}
