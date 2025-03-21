package config

import (
	"fmt"
	"log"
	"strconv"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
)

// CheckWhitelist checks if an event meets the whitelist criteria.
func CheckWhitelist(evt nostr.Event) (bool, string) {
	whitelistCfg := GetWhitelistConfig()
	if whitelistCfg == nil {
		return false, "Internal server error: whitelist configuration is missing"
	}

	// Check if the event's kind is whitelisted
	if whitelistCfg.KindWhitelist.Enabled && !IsKindWhitelisted(evt.Kind) {
		return false, "not allowed: event kind is not whitelisted"
	}

	// Check if the event's pubkey is whitelisted
	if whitelistCfg.PubkeyWhitelist.Enabled && !IsPubKeyWhitelisted(evt.PubKey, false) {
		return false, "not allowed: pubkey or npub is not whitelisted"
	}

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
			log.Printf("Error decoding npub: %v", err)
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
			log.Printf("Error fetching pubkeys from domains: %v", err)
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
			fmt.Println("Error converting whitelisted kind to int:", err)
			continue
		}
		if kind == whitelistedKind {
			return true
		}
	}

	return false
}
