package utils

import (
	"fmt"
	"grain/config"
	cfg "grain/config/types"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
)

// CheckBlacklist checks if a pubkey is in the blacklist based on event content
func CheckBlacklist(pubkey, eventContent string) (bool, string) {
    cfg := config.GetConfig().Blacklist

    if !cfg.Enabled {
        return false, ""
    }

    log.Printf("Checking blacklist for pubkey: %s", pubkey)

    // Check for permanent blacklist by pubkey or npub
    if isPubKeyPermanentlyBlacklisted(pubkey) {
        log.Printf("Pubkey %s is permanently blacklisted", pubkey)
        return true, fmt.Sprintf("pubkey %s is permanently blacklisted", pubkey)
    }

    // Check for temporary ban
    if isPubKeyTemporarilyBlacklisted(pubkey) {
        log.Printf("Pubkey %s is temporarily blacklisted", pubkey)
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
        log.Printf("Pubkey %s not found in temporary blacklist", pubkey)
        return false
    }

    now := time.Now()
    if now.After(entry.unbanTime) {
        log.Printf("Temporary ban for pubkey %s has expired. Count: %d", pubkey, entry.count)
        return false
    }

    log.Printf("Pubkey %s is currently temporarily blacklisted. Count: %d, Unban time: %s", pubkey, entry.count, entry.unbanTime)
    return true
}

func ClearTemporaryBans() {
	mu.Lock()
	defer mu.Unlock()
	tempBannedPubkeys = make(map[string]*tempBanEntry)
}

var (
    tempBannedPubkeys = make(map[string]*tempBanEntry)
    mu sync.Mutex
)

type tempBanEntry struct {
    count     int
    unbanTime time.Time
}

// Adds a pubkey to the temporary blacklist
func AddToTemporaryBlacklist(pubkey string) error {
    mu.Lock()
    defer mu.Unlock()

    cfg := config.GetConfig().Blacklist

    entry, exists := tempBannedPubkeys[pubkey]
    if !exists {
        log.Printf("Creating new temporary ban entry for pubkey %s", pubkey)
        entry = &tempBanEntry{
            count:     0,
            unbanTime: time.Now(),
        }
        tempBannedPubkeys[pubkey] = entry
    } else {
        log.Printf("Updating existing temporary ban entry for pubkey %s. Current count: %d", pubkey, entry.count)
        if time.Now().After(entry.unbanTime) {
            log.Printf("Previous ban for pubkey %s has expired. Keeping count at %d", pubkey, entry.count)
        }
    }

    // Increment the count
    entry.count++
    entry.unbanTime = time.Now().Add(time.Duration(cfg.TempBanDuration) * time.Second)

    log.Printf("Pubkey %s temporary ban count updated to: %d, MaxTempBans: %d, New unban time: %s", pubkey, entry.count, cfg.MaxTempBans, entry.unbanTime)

    if entry.count > cfg.MaxTempBans {
        log.Printf("Attempting to move pubkey %s to permanent blacklist", pubkey)
        delete(tempBannedPubkeys, pubkey)
        
        // Release the lock before calling AddToPermanentBlacklist
        mu.Unlock()
        err := AddToPermanentBlacklist(pubkey)
        mu.Lock() // Re-acquire the lock
        
        if err != nil {
            log.Printf("Error adding pubkey %s to permanent blacklist: %v", pubkey, err)
            return err
        }
        log.Printf("Successfully added pubkey %s to permanent blacklist", pubkey)
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
    // Remove the mutex lock from here
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

