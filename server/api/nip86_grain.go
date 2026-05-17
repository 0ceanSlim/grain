// NIP-86 phase 2 grain_* vendor methods.
//
// Lifecycle pattern is uniform across the section-update methods:
//
//   1. Pull params[0] which holds the FULL section blob as a JSON
//      object. Marshal back to bytes, unmarshal into the typed
//      config struct. This is fiddlier than positional simple
//      values but lets the dashboard send "the whole rate_limit
//      block at once" with one round trip and no per-field
//      bikeshedding.
//   2. Validate (where validators exist).
//   3. Call the config.Update<section> helper which writes the file
//      with watcher suppression — fsnotify won't fire a restart on
//      our own write; the dashboard's Apply button explicitly
//      triggers one via grain_reloadconfig.
//   4. Return `{result: {ok:true, restart_pending:true}, error:""}`.
//
// Cosmetics: every result uses the same map shape so the dashboard
// has one decoder for the whole grain_update* family.

package api

import (
	"encoding/json"
	"fmt"

	"github.com/0ceanslim/grain/config"
	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// paramJSON extracts params[0] and decodes it into the supplied
// destination via a marshal → unmarshal round-trip. The dispatcher
// receives params as []any (default JSON decoding) so we can't pass
// the original bytes directly; the round-trip is cheap and lets
// the typed struct catch shape errors per-field instead of us
// hand-walking the map.
func paramJSON(params []any, i int, dst any) error {
	if i >= len(params) {
		return fmt.Errorf("missing params[%d]", i)
	}
	raw, err := json.Marshal(params[i])
	if err != nil {
		return fmt.Errorf("re-marshal params[%d]: %w", i, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decode params[%d]: %w", i, err)
	}
	return nil
}

// updateResult is the canonical staged-update response. The
// dashboard reads `restart_pending` and surfaces an "Apply changes"
// affordance until grain_reloadconfig is called.
func updateResult() map[string]any {
	return map[string]any{
		"ok":              true,
		"restart_pending": true,
	}
}

// ─── server / rate_limit / event_purge / logging / auth / etc. ───

func runUpdateServer(params []any, signer string) (any, string) {
	var srv cfgType.ServerSettings
	if err := paramJSON(params, 0, &srv); err != nil {
		return nil, err.Error()
	}
	if err := config.UpdateServerConfig(srv); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 grain_updateserver", "signer", signer)
	return updateResult(), ""
}

func runUpdateRateLimit(params []any, signer string) (any, string) {
	var rl cfgType.RateLimitConfig
	if err := paramJSON(params, 0, &rl); err != nil {
		return nil, err.Error()
	}
	if err := config.UpdateRateLimitConfig(rl); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 grain_updateratelimit", "signer", signer)
	return updateResult(), ""
}

func runUpdateEventPurge(params []any, signer string) (any, string) {
	var ep cfgType.EventPurgeConfig
	if err := paramJSON(params, 0, &ep); err != nil {
		return nil, err.Error()
	}
	if err := config.UpdateEventPurgeConfig(ep); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 grain_updateeventpurge", "signer", signer)
	return updateResult(), ""
}

func runUpdateLogging(params []any, signer string) (any, string) {
	var lg cfgType.LogConfig
	if err := paramJSON(params, 0, &lg); err != nil {
		return nil, err.Error()
	}
	if err := config.UpdateLoggingConfig(lg); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 grain_updatelogging", "signer", signer)
	return updateResult(), ""
}

func runUpdateAuth(params []any, signer string) (any, string) {
	var au cfgType.AuthConfig
	if err := paramJSON(params, 0, &au); err != nil {
		return nil, err.Error()
	}
	if err := config.UpdateAuthConfig(au); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 grain_updateauth", "signer", signer)
	return updateResult(), ""
}

func runUpdateBackupRelay(params []any, signer string) (any, string) {
	var br cfgType.BackupRelayConfig
	if err := paramJSON(params, 0, &br); err != nil {
		return nil, err.Error()
	}
	if err := config.UpdateBackupRelayConfig(br); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 grain_updatebackuprelay", "signer", signer, "enabled", br.Enabled)
	return updateResult(), ""
}

func runUpdateResourceLimits(params []any, signer string) (any, string) {
	var rl cfgType.ResourceLimits
	if err := paramJSON(params, 0, &rl); err != nil {
		return nil, err.Error()
	}
	if err := config.UpdateResourceLimits(rl); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 grain_updateresourcelimits", "signer", signer)
	return updateResult(), ""
}

func runUpdateEventTimeConstraints(params []any, signer string) (any, string) {
	var etc cfgType.EventTimeConstraints
	if err := paramJSON(params, 0, &etc); err != nil {
		return nil, err.Error()
	}
	if err := config.UpdateEventTimeConstraints(etc); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 grain_updateeventtimeconstraints", "signer", signer)
	return updateResult(), ""
}

func runUpdateWhitelistConfig(params []any, signer string) (any, string) {
	var wl cfgType.WhitelistConfig
	if err := paramJSON(params, 0, &wl); err != nil {
		return nil, err.Error()
	}
	if err := config.UpdateWhitelistConfig(wl); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 grain_updatewhitelistconfig", "signer", signer)
	// Whitelist edits take effect via cache refresh — but the
	// `enabled` toggle is read from the live struct on every event
	// validation, so it's already live too. Still report
	// restart_pending so the dashboard surfaces "click Apply for a
	// clean state" if the operator wants it.
	return updateResult(), ""
}

func runUpdateBlacklistConfig(params []any, signer string) (any, string) {
	var bl cfgType.BlacklistConfig
	if err := paramJSON(params, 0, &bl); err != nil {
		return nil, err.Error()
	}
	if err := config.UpdateBlacklistConfig(bl); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 grain_updateblacklistconfig", "signer", signer)
	return updateResult(), ""
}

// ─── operational ─────────────────────────────────────────────────

func runReloadConfig(signer string) (any, string) {
	log.RelayAPI().Info("NIP-86 grain_reloadconfig", "signer", signer)
	// Fire the restart asynchronously so the HTTP response gets
	// out before the server starts tearing down. The restart loop
	// pauses 3 s before bringing the new instance up; the
	// dashboard polls /api/v1/session to know when the relay is
	// back.
	go config.TriggerRestart()
	return map[string]any{
		"ok":                 true,
		"restart_in_seconds": 4, // 1s fsnotify debounce + ~3s pause
	}, ""
}

func runRefreshCache(signer string) (any, string) {
	cache := config.GetPubkeyCache()
	if cache == nil {
		return nil, "pubkey cache not initialized"
	}
	if err := cache.RefreshWhitelist(); err != nil {
		log.RelayAPI().Warn("NIP-86 grain_refreshcache: whitelist refresh failed", "error", err)
	}
	if err := cache.RefreshBlacklist(); err != nil {
		log.RelayAPI().Warn("NIP-86 grain_refreshcache: blacklist refresh failed", "error", err)
	}
	log.RelayAPI().Info("NIP-86 grain_refreshcache", "signer", signer)
	return cache.GetPubkeyCacheStats(), ""
}

// ─── reads ───────────────────────────────────────────────────────

func runGetWhitelistConfig() (any, string) {
	cfg := config.GetWhitelistConfig()
	if cfg == nil {
		return nil, "whitelist configuration is not loaded"
	}
	return cfg, ""
}

// runGetBlacklistConfig returns blacklist.yml's struct with the IP
// fields overlaid from config.yml's blacklist: section, so the
// dashboard sees a single coherent shape. Writes via
// grain_updateblacklistconfig deliberately drop those IP fields (see
// config.UpdateBlacklistConfig) — per-IP edits go through blockip /
// unblockip, which target config.yml directly.
func runGetBlacklistConfig() (any, string) {
	cfg := config.GetBlacklistConfig()
	if cfg == nil {
		return nil, "blacklist configuration is not loaded"
	}
	out := *cfg
	if sc := config.GetConfig(); sc != nil {
		out.PermanentBlockedIPs = sc.Blacklist.PermanentBlockedIPs
		out.IPMaxTempBans = sc.Blacklist.IPMaxTempBans
		out.IPTempBanDuration = sc.Blacklist.IPTempBanDuration
		out.IPRateViolationThreshold = sc.Blacklist.IPRateViolationThreshold
	}
	return out, ""
}
