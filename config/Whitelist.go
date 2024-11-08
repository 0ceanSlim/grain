package config

import (
	"fmt"
	nostr "grain/server/types"
	"grain/server/utils"
	"strconv"
)

// CheckWhitelist checks if an event meets the whitelist criteria.
func CheckWhitelist(evt nostr.Event) (bool, string) {
    // Get the current whitelist configuration
    whitelistCfg := GetWhitelistConfig()
    if whitelistCfg == nil {
        return false, "Internal server error: whitelist configuration is missing"
    }

    // If domain whitelisting is enabled, fetch pubkeys from domains
    if whitelistCfg.DomainWhitelist.Enabled {
        domains := whitelistCfg.DomainWhitelist.Domains
        pubkeys, err := utils.FetchPubkeysFromDomains(domains)
        if err != nil {
            return false, "Error fetching pubkeys from domains"
        }
        // Update the whitelisted pubkeys dynamically
        whitelistCfg.PubkeyWhitelist.Pubkeys = append(whitelistCfg.PubkeyWhitelist.Pubkeys, pubkeys...)
    }

    // Check if the event's kind is whitelisted
    if whitelistCfg.KindWhitelist.Enabled && !IsKindWhitelisted(evt.Kind) {
        return false, "not allowed: event kind is not whitelisted"
    }

    // Check if the event's pubkey is whitelisted
    if whitelistCfg.PubkeyWhitelist.Enabled && !IsPubKeyWhitelisted(evt.PubKey) {
        return false, "not allowed: pubkey or npub is not whitelisted"
    }

    return true, ""
}

// Check if a pubkey or npub is whitelisted
func IsPubKeyWhitelisted(pubKey string) bool {
    cfg := GetWhitelistConfig()
    if !cfg.PubkeyWhitelist.Enabled {
        return true
    }

    for _, whitelistedKey := range cfg.PubkeyWhitelist.Pubkeys {
        if pubKey == whitelistedKey {
            return true
        }
    }

    for _, npub := range cfg.PubkeyWhitelist.Npubs {
        decodedPubKey, err := utils.DecodeNpub(npub)
        if err != nil {
            fmt.Println("Error decoding npub:", err)
            continue
        }
        if pubKey == decodedPubKey {
            return true
        }
    }

    return false
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