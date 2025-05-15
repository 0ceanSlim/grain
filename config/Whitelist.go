package config

import (
	"strconv"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
)

func CheckWhitelist(evt nostr.Event) (bool, string) {
	whitelistCfg := GetWhitelistConfig()
	if whitelistCfg == nil {
		log.Error("Whitelist configuration is missing")
		return false, "Internal server error: whitelist configuration is missing"
	}

	// Check if the event's kind is whitelisted
	if whitelistCfg.KindWhitelist.Enabled && !IsKindWhitelisted(evt.Kind) {
		log.Warn("Event kind is not whitelisted", "kind", evt.Kind)
		return false, "not allowed: event kind is not whitelisted"
	}

	// Check if the event's pubkey is whitelisted
	if whitelistCfg.PubkeyWhitelist.Enabled && !IsPubKeyWhitelisted(evt.PubKey, false) {
		log.Warn("Pubkey is not whitelisted", "pubkey", evt.PubKey)
		return false, "not allowed: pubkey or npub is not whitelisted"
	}

	log.Debug("Whitelist check passed", "kind", evt.Kind, "pubkey", evt.PubKey)
	return true, ""
}

// IsPubKeyWhitelisted checks if a pubkey or npub is whitelisted, considering pubkeys from domains.
// The `skipEnabledCheck` flag indicates if the check should happen regardless of whether or not
// the whitelist is enabled.
func IsPubKeyWhitelisted(pubKey string, skipEnabledCheck bool) bool {
	cfg := GetWhitelistConfig()
	if cfg == nil {
		return false // No configuration means no whitelisting.
	}

	// If the whitelist is disabled but this check is for purging, we still evaluate it.
	if !cfg.PubkeyWhitelist.Enabled && !skipEnabledCheck {
		return true // Whitelisting is not enforced for posting if disabled.
	}

	// Check statically defined pubkeys
	for _, whitelistedKey := range cfg.PubkeyWhitelist.Pubkeys {
		if pubKey == whitelistedKey {
			return true
		}
	}

	// Check statically defined npubs after decoding them to pubkeys
	for _, npub := range cfg.PubkeyWhitelist.Npubs {
		decodedPubKey, err := utils.DecodeNpub(npub)
		if err != nil {
			log.Error("Failed to decode npub", "npub", npub, "error", err)
			continue
		}
		if pubKey == decodedPubKey {
			return true
		}
	}

	// Always fetch and check pubkeys from domains if skipEnabledCheck is true
	if cfg.DomainWhitelist.Enabled || skipEnabledCheck {
		domains := cfg.DomainWhitelist.Domains
		pubkeys, err := utils.FetchPubkeysFromDomains(domains)
		if err != nil {
			log.Error("Failed to fetch pubkeys from domains", "domains", domains, "error", err)
			return false // Consider errors as non-whitelisted for purging
		}

		for _, domainPubKey := range pubkeys {
			if pubKey == domainPubKey {
				return true
			}
		}
	}

	return false // Not whitelisted
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
			log.Error("Failed to convert whitelisted kind to int", "kind", whitelistedKindStr, "error", err)
			continue
		}
		if kind == whitelistedKind {
			return true
		}
	}

	return false
}
