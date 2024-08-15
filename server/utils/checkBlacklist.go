package utils

import (
	"fmt"
	"grain/config"
	cfg "grain/config/types"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
)

// Structure to manage temporary bans with timestamps
type tempBanEntry struct {
	count     int       // Number of temporary bans
	unbanTime time.Time // Time when the pubkey should be unbanned
}

var (
	tempBannedPubkeys = make(map[string]*tempBanEntry)
	mu                sync.Mutex
)

func ClearTemporaryBans() {
	mu.Lock()
	defer mu.Unlock()
	tempBannedPubkeys = make(map[string]*tempBanEntry)
}


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

	// Check for temporary ban
	if isPubKeyTemporarilyBlacklisted(pubkey) {
		return true, fmt.Sprintf("pubkey %s is temporarily blacklisted", pubkey)
	}

	// Check for permanent ban based on wordlist
	for _, word := range cfg.PermanentBanWords {
		if strings.Contains(eventContent, word) {
			err := AddToPermanentBlacklist(pubkey)
			if err != nil {
				return true, fmt.Sprintf("pubkey %s is permanently banned and failed to save: %v", pubkey, err)
			}
			return true, fmt.Sprintf("pubkey %s is permanently banned for containing forbidden words", pubkey)
		}
	}

	// Check for temporary ban based on wordlist
	for _, word := range cfg.TempBanWords {
		if strings.Contains(eventContent, word) {
			err := AddToTemporaryBlacklist(pubkey)
			if err != nil {
				return true, fmt.Sprintf("pubkey %s is temporarily banned and failed to save: %v", pubkey, err)
			}
			return true, fmt.Sprintf("pubkey %s is temporarily banned for containing forbidden words", pubkey)
		}
	}

	return false, ""
}


// Checks if a pubkey is temporarily blacklisted
func isPubKeyTemporarilyBlacklisted(pubkey string) bool {
	mu.Lock()
	defer mu.Unlock()

	entry, exists := tempBannedPubkeys[pubkey]
	if !exists {
		return false
	}

	// If the ban has expired, remove it from the temporary ban list
	if time.Now().After(entry.unbanTime) {
		delete(tempBannedPubkeys, pubkey)
		return false
	}

	return true
}

// Adds a pubkey to the temporary blacklist
func AddToTemporaryBlacklist(pubkey string) error {
	mu.Lock()
	defer mu.Unlock()

	cfg := config.GetConfig().Blacklist

	// Check if the pubkey is already temporarily banned
	entry, exists := tempBannedPubkeys[pubkey]
	if !exists {
		entry = &tempBanEntry{
			count:     1,
			unbanTime: time.Now().Add(time.Duration(cfg.TempBanDuration) * time.Second),
		}
		tempBannedPubkeys[pubkey] = entry
	}

	// Increment the temporary ban count and set the unban time
	entry.count++
	entry.unbanTime = time.Now().Add(time.Duration(cfg.TempBanDuration) * time.Second)

	// If the count exceeds max_temp_bans, move to permanent blacklist
	if entry.count >= cfg.MaxTempBans {
		delete(tempBannedPubkeys, pubkey)
		return AddToPermanentBlacklist(pubkey)
	}

	return nil
}

// Checks if a pubkey is permanently blacklisted (only using config.yml)
func isPubKeyPermanentlyBlacklisted(pubKey string) bool {
	cfg := config.GetConfig().Blacklist // Get the latest configuration

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
	mu.Lock()
	defer mu.Unlock()

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
	configData := config.GetConfig()
	configData.Blacklist = blacklistConfig

	data, err := yaml.Marshal(configData)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	err = os.WriteFile("config.yml", data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config to file: %v", err)
	}

	return nil
}

