package utils

import (
	"fmt"
	"grain/config"
	"strconv"
)

// Helper function to check if a pubkey or npub is whitelisted
func IsPubKeyWhitelisted(pubKey string) bool {
	cfg := config.GetConfig()
	if !cfg.PubkeyWhitelist.Enabled {
		return true
	}

	// Check pubkeys
	for _, whitelistedKey := range cfg.PubkeyWhitelist.Pubkeys {
		if pubKey == whitelistedKey {
			return true
		}
	}

	// Check npubs
	for _, npub := range cfg.PubkeyWhitelist.Npubs {
		decodedPubKey, err := DecodeNpub(npub)
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

func IsKindWhitelisted(kind int) bool {
	cfg := config.GetConfig()
	if !cfg.KindWhitelist.Enabled {
		return true
	}

	// Check event kinds
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
