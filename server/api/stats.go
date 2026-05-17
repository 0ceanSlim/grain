// NIP-86 stats helpers (grain_stats_overview).
//
// Counters that live inside `package server` (currentConnections,
// totalMessagesSent, uptime since process start) can't be imported
// here without a cycle — server already depends on server/api for
// the dispatcher. Instead startup.go installs a hook function via
// SetServerStatsHook, and grain_stats_overview merges that with
// the stats accessible from inside this package (cache age,
// config-driven counts).
//
// If the hook isn't wired (tests running server/api in isolation),
// the server-side counters are reported as zero and the response
// still has the version + config-driven fields.

package api

import (
	"github.com/0ceanslim/grain/config"
)

// ServerStats is the slice of stats only the server package can
// produce. Kept small on purpose — every field added here has to
// thread through the hook.
type ServerStats struct {
	ActiveConnections int64  `json:"active_connections"`
	TotalMessagesSent int64  `json:"total_messages_sent"`
	UptimeSeconds     int64  `json:"uptime_seconds"`
	Version           string `json:"version"`
	BuildTime         string `json:"build_time"`
	GitCommit         string `json:"git_commit"`
}

// statsHook is installed at startup by SetServerStatsHook. Default
// returns zero values so tests that don't wire the hook still get
// a meaningful response.
var statsHook func() ServerStats = func() ServerStats { return ServerStats{} }

// SetServerStatsHook is wired from server/startup.go once the
// counters are available. The hook is called per grain_stats_overview
// request — should be cheap (atomic loads).
func SetServerStatsHook(fn func() ServerStats) {
	if fn != nil {
		statsHook = fn
	}
}

// statsOverviewResult is what grain_stats_overview returns.
// Single bundled response so the dashboard can render an overview
// card with one round trip. Sub-methods (grain_stats_connections,
// grain_stats_events, grain_stats_database) can split later if
// finer granularity is needed.
type statsOverviewResult struct {
	Server    ServerStats     `json:"server"`
	Whitelist statsListCounts `json:"whitelist"`
	Blacklist statsListCounts `json:"blacklist"`
	Cache     map[string]any  `json:"cache"`
}

type statsListCounts struct {
	Enabled  bool `json:"enabled"`
	Pubkeys  int  `json:"pubkeys"`
	Npubs    int  `json:"npubs"`
	Domains  int  `json:"domains,omitempty"`
	Kinds    int  `json:"kinds,omitempty"`
	IPBlocks int  `json:"ip_blocks,omitempty"`
}

// gatherStatsOverview composes the response. Cheap: just reads
// from atomic counters and existing in-memory config / cache.
func gatherStatsOverview() statsOverviewResult {
	res := statsOverviewResult{
		Server: statsHook(),
	}

	if wl := config.GetWhitelistConfig(); wl != nil {
		res.Whitelist = statsListCounts{
			Enabled: wl.PubkeyWhitelist.Enabled,
			Pubkeys: len(wl.PubkeyWhitelist.Pubkeys),
			Npubs:   len(wl.PubkeyWhitelist.Npubs),
			Domains: len(wl.DomainWhitelist.Domains),
			Kinds:   len(wl.KindWhitelist.Kinds),
		}
	}

	if bl := config.GetBlacklistConfig(); bl != nil {
		res.Blacklist = statsListCounts{
			Enabled:  bl.Enabled,
			Pubkeys:  len(bl.PermanentBlacklistPubkeys),
			Npubs:    len(bl.PermanentBlacklistNpubs),
			IPBlocks: len(bl.PermanentBlockedIPs),
		}
	}

	if pc := config.GetPubkeyCache(); pc != nil {
		// GetPubkeyCacheStats returns a map[string]interface{}
		// covering cache sizes + age timestamps. Including it
		// verbatim keeps the response future-proof — the dashboard
		// renders whatever it finds.
		res.Cache = pc.GetPubkeyCacheStats()
	}

	return res
}
