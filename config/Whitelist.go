package config

import (
	"fmt"
	"grain/server/utils"
	"strconv"
)

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