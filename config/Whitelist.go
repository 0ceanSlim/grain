package config

import (
	"strconv"

	nostr "github.com/0ceanslim/grain/server/types"
)

// CheckWhitelistCached uses cached pubkey lists instead of real-time lookups
func CheckWhitelistCached(evt nostr.Event) (bool, string) {
	whitelistCfg := GetWhitelistConfig()
	if whitelistCfg == nil {
		configLog().Error("Whitelist configuration is missing")
		return false, "Internal server error: whitelist configuration is missing"
	}

	pubkeyCache := GetPubkeyCache()

	// Check if the event's kind is whitelisted (no caching needed for this)
	if whitelistCfg.KindWhitelist.Enabled && !IsKindWhitelisted(evt.Kind) {
		configLog().Warn("Event kind is not whitelisted", "kind", evt.Kind)
		return false, "not allowed: event kind is not whitelisted"
	}

	// Check if the event's pubkey is whitelisted using cache
	if whitelistCfg.PubkeyWhitelist.Enabled && !pubkeyCache.IsWhitelisted(evt.PubKey) {
		configLog().Warn("Pubkey is not whitelisted", "pubkey", evt.PubKey)
		return false, "not allowed: pubkey or npub is not whitelisted"
	}

	configLog().Debug("Whitelist check passed", "kind", evt.Kind, "pubkey", evt.PubKey)
	return true, ""
}

// IsPubKeyWhitelistedCached checks cache instead of real-time lookups
// with support for skipEnabledCheck parameter for purging operations
func IsPubKeyWhitelistedCached(pubKey string, skipEnabledCheck bool) bool {
	cfg := GetWhitelistConfig()
	if cfg == nil {
		return false
	}

	// If the whitelist is disabled but this check is for purging, we still evaluate it.
	if !cfg.PubkeyWhitelist.Enabled && !skipEnabledCheck {
		return true // Whitelisting is not enforced for posting if disabled.
	}

	// Use cached result - the cache already includes all sources (pubkeys, npubs, domains)
	return GetPubkeyCache().IsWhitelisted(pubKey)
}

// Check if a kind is whitelisted
func IsKindWhitelisted(kind int) bool {
	cfg := GetWhitelistConfig()
	if !cfg.KindWhitelist.Enabled {
		return true
	}

	for _, whitelistedKindStr := range cfg.KindWhitelist.Kinds {
		whitelistedKind, err := strconv.Atoi(whitelistedKindStr)
		if err != nil {
			configLog().Error("Failed to convert whitelisted kind to int", "kind", whitelistedKindStr, "error", err)
			continue
		}
		if kind == whitelistedKind {
			return true
		}
	}

	return false
}
