package config

import (
	"fmt"
	nostr "grain/server/types"
	"grain/server/utils"
	"log"
	"strconv"
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
    if whitelistCfg.PubkeyWhitelist.Enabled && !IsPubKeyWhitelisted(evt.PubKey) {
        return false, "not allowed: pubkey or npub is not whitelisted"
    }

    return true, ""
}

// IsPubKeyWhitelisted checks if a pubkey or npub is whitelisted, considering pubkeys from domains.
func IsPubKeyWhitelisted(pubKey string) bool {
    cfg := GetWhitelistConfig()
    if cfg == nil || !cfg.PubkeyWhitelist.Enabled {
        return true // Whitelisting is not enforced if the configuration is missing or disabled
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

    // Fetch and check pubkeys from domains if domain whitelisting is enabled
    if cfg.DomainWhitelist.Enabled {
        domains := cfg.DomainWhitelist.Domains
        pubkeys, err := utils.FetchPubkeysFromDomains(domains)
        if err != nil {
            log.Printf("Error fetching pubkeys from domains: %v", err)
            return false // Consider returning true or handling based on your application's needs
        }

        for _, domainPubKey := range pubkeys {
            if pubKey == domainPubKey {
                return true
            }
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