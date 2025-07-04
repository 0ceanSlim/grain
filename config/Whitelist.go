package config

import (
	"strconv"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// CheckWhitelistCached uses cached pubkey lists and respects enabled state for validation
func CheckWhitelistCached(evt nostr.Event) (bool, string) {
	whitelistCfg := GetWhitelistConfig()
	if whitelistCfg == nil {
		log.Config().Error("Whitelist configuration is missing")
		return false, "Internal server error: whitelist configuration is missing"
	}

	// Check if the event's kind is whitelisted (no caching needed for this)
	if whitelistCfg.KindWhitelist.Enabled && !IsKindWhitelisted(evt.Kind) {
		log.Config().Warn("Event kind is not whitelisted", "kind", evt.Kind)
		return false, "not allowed: event kind is not whitelisted"
	}

	// Check if the event's pubkey is whitelisted using cache with enabled state check
	pubkeyCache := GetPubkeyCache()
	if whitelistCfg.PubkeyWhitelist.Enabled && !pubkeyCache.IsWhitelistedForValidation(evt.PubKey) {
		log.Config().Warn("Pubkey is not whitelisted", "pubkey", evt.PubKey)
		return false, "not allowed: pubkey or npub is not whitelisted"
	}

	log.Config().Debug("Whitelist check passed", "kind", evt.Kind, "pubkey", evt.PubKey)
	return true, ""
}

// IsPubKeyWhitelistedCached for purging operations - always uses cache regardless of enabled state
func IsPubKeyWhitelistedCached(pubKey string, skipEnabledCheck bool) bool {
	pubkeyCache := GetPubkeyCache()

	if skipEnabledCheck {
		// For purging operations - use cache regardless of enabled state
		return pubkeyCache.IsWhitelisted(pubKey)
	}

	// For validation operations - respect enabled state
	return pubkeyCache.IsWhitelistedForValidation(pubKey)
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
			log.Config().Error("Failed to convert whitelisted kind to int", "kind", whitelistedKindStr, "error", err)
			continue
		}
		if kind == whitelistedKind {
			return true
		}
	}

	return false
}
