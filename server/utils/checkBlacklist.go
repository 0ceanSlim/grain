package utils

import (
	"fmt"
	"grain/config"
	cfg "grain/config/types"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// CheckBlacklist checks if a pubkey is in the blacklist based on event content
func CheckBlacklist(pubkey, eventContent string) (bool, string) {
	cfg := config.GetConfig().Blacklist

	if !cfg.Enabled {
		return false, ""
	}

	// Check for permanent blacklist by pubkey or npub
	if isPubKeyPermanentlyBlacklisted(pubkey) {
		return true, fmt.Sprintf("pubkey %s is permanently blacklisted", pubkey)
	}

	// Check for permanent ban based on wordlist
	for _, word := range cfg.PermanentBanWords {
		if strings.Contains(eventContent, word) {
			// Permanently ban the pubkey
			err := AddToPermanentBlacklist(pubkey)
			if err != nil {
				return true, fmt.Sprintf("pubkey %s is permanently banned and failed to save: %v", pubkey, err)
			}
			return true, fmt.Sprintf("pubkey %s is permanently banned for containing forbidden words", pubkey)
		}
	}

	return false, ""
}

func isPubKeyPermanentlyBlacklisted(pubKey string) bool {
	cfg := config.GetConfig().Blacklist
	if !cfg.Enabled {
		return false
	}

	// Check pubkeys
	for _, blacklistedKey := range cfg.PermanentBlacklistPubkeys {
		if pubKey == blacklistedKey {
			return true
		}
	}

	// Check npubs
	for _, npub := range cfg.PermanentBlacklistNpubs {
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

func AddToPermanentBlacklist(pubkey string) error {
	cfg := config.GetConfig().Blacklist

	// Check if already blacklisted
	if isPubKeyPermanentlyBlacklisted(pubkey) {
		return fmt.Errorf("pubkey %s is already in the permanent blacklist", pubkey)
	}

	// Add pubkey to the blacklist
	cfg.PermanentBlacklistPubkeys = append(cfg.PermanentBlacklistPubkeys, pubkey)

	// Persist changes to config.yml
	return saveBlacklistConfig(cfg)
}

func saveBlacklistConfig(blacklistConfig cfg.BlacklistConfig) error {
	cfg := config.GetConfig()
	cfg.Blacklist = blacklistConfig

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	err = os.WriteFile("config.yml", data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config to file: %v", err)
	}

	return nil
}
