package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	types "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils"

	"golang.org/x/net/websocket"
	"gopkg.in/yaml.v3"
)

// CheckBlacklistCached uses cached pubkey lists and respects enabled state for validation
func CheckBlacklistCached(pubkey, eventContent string) (bool, string) {
	blacklistConfig := GetBlacklistConfig()
	if blacklistConfig == nil || !blacklistConfig.Enabled {
		return false, ""
	}

	configLog().Debug("Checking cached blacklist for pubkey", "pubkey", pubkey)

	pubkeyCache := GetPubkeyCache()

	// Check cached permanent blacklist (respects enabled state for validation)
	if pubkeyCache.IsBlacklistedForValidation(pubkey) {
		configLog().Warn("Pubkey found in cached blacklist", "pubkey", pubkey)
		return true, "blocked: pubkey is blacklisted"
	}

	// Check for temporary ban (this still needs real-time checking)
	if isPubKeyTemporarilyBlacklisted(pubkey) {
		configLog().Warn("Pubkey temporarily blacklisted", "pubkey", pubkey)
		return true, "blocked: pubkey is temporarily blacklisted"
	}

	// Check for permanent ban based on content (wordlist)
	for _, word := range blacklistConfig.PermanentBanWords {
		if strings.Contains(eventContent, word) {
			err := AddToPermanentBlacklist(pubkey)
			if err != nil {
				configLog().Error("Failed to add pubkey to permanent blacklist", 
					"pubkey", pubkey, 
					"word", word, 
					"error", err)
				return true, fmt.Sprintf("pubkey %s is permanently banned and failed to save: %v", pubkey, err)
			}
			
			// Trigger immediate blacklist refresh to include this pubkey
			go GetPubkeyCache().RefreshBlacklist()
			
			configLog().Warn("Pubkey permanently banned due to wordlist match", 
				"pubkey", pubkey, 
				"word", word)
			return true, "blocked: pubkey is permanently banned"
		}
	}

	// Check for temporary ban based on content (wordlist)
	for _, word := range blacklistConfig.TempBanWords {
		if strings.Contains(eventContent, word) {
			err := AddToTemporaryBlacklist(pubkey, *blacklistConfig)
			if err != nil {
				configLog().Error("Failed to add pubkey to temporary blacklist", 
					"pubkey", pubkey, 
					"word", word, 
					"error", err)
				return true, fmt.Sprintf("pubkey %s is temporarily banned and failed to save: %v", pubkey, err)
			}
			configLog().Warn("Pubkey temporarily banned due to wordlist match", 
				"pubkey", pubkey, 
				"word", word)
			return true, "blocked: pubkey is temporarily banned"
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
		// Pubkey not found in temporary blacklist
		configLog().Debug("Pubkey not in temporary blacklist", "pubkey", pubkey)
		return false
	}

	now := time.Now()
	if now.After(entry.unbanTime) {
		// Temporary ban expired
		configLog().Info("Temporary ban expired", 
			"pubkey", pubkey, 
			"count", entry.count,
			"unban_time", entry.unbanTime.Format(time.RFC3339))
		return false
	}

	// Pubkey currently blacklisted
	configLog().Warn("Pubkey currently temporarily blacklisted", 
		"pubkey", pubkey, 
		"count", entry.count,
		"unban_time", entry.unbanTime.Format(time.RFC3339))
	return true
}

func ClearTemporaryBans() {
	mu.Lock()
	defer mu.Unlock()
	tempBannedPubkeys = make(map[string]*tempBanEntry)
	configLog().Debug("Cleared all temporary bans", "timestamp", time.Now().Format(time.RFC3339))
}

var (
	tempBannedPubkeys = make(map[string]*tempBanEntry)
)

type tempBanEntry struct {
	count     int
	unbanTime time.Time
}

// Adds a pubkey to the temporary blacklist
func AddToTemporaryBlacklist(pubkey string, blacklistConfig types.BlacklistConfig) error {
	mu.Lock()
	defer mu.Unlock()

	entry, exists := tempBannedPubkeys[pubkey]
	if !exists {
		// Creating a new temp ban entry
		configLog().Info("Creating new temporary ban entry", "pubkey", pubkey)
		entry = &tempBanEntry{
			count:     0,
			unbanTime: time.Now(),
		}
		tempBannedPubkeys[pubkey] = entry
	} else {
		// Updating an existing temp ban entry
		configLog().Info("Updating existing temporary ban entry", 
			"pubkey", pubkey, 
			"current_count", entry.count)

		if time.Now().After(entry.unbanTime) {
			configLog().Info("Previous ban expired, keeping count", 
				"pubkey", pubkey, 
				"count", entry.count)
		}
	}

	// Increment the count
	entry.count++
	entry.unbanTime = time.Now().Add(time.Duration(blacklistConfig.TempBanDuration) * time.Second)

	// Updating temp ban count
	configLog().Debug("Updated temporary ban", 
		"pubkey", pubkey, 
		"count", entry.count,
		"max_temp_bans", blacklistConfig.MaxTempBans,
		"unban_time", entry.unbanTime.Format(time.RFC3339))

	if entry.count > blacklistConfig.MaxTempBans {
		// Attempting to move to permanent blacklist
		configLog().Warn("Max temporary bans exceeded, moving to permanent blacklist", 
			"pubkey", pubkey,
			"count", entry.count)

		delete(tempBannedPubkeys, pubkey)

		// Release the lock before calling AddToPermanentBlacklist
		mu.Unlock()
		err := AddToPermanentBlacklist(pubkey)
		mu.Lock() // Re-acquire the lock

		if err != nil {
			// Error adding to permanent blacklist
			configLog().Error("Failed to move pubkey to permanent blacklist", 
				"pubkey", pubkey, 
				"error", err)
			return err
		}
		// Successfully added to permanent blacklist
		configLog().Info("Successfully moved pubkey to permanent blacklist", "pubkey", pubkey)
	}

	return nil
}

// GetTemporaryBlacklist fetches all currently active temporary bans
func GetTemporaryBlacklist() []map[string]interface{} {
	mu.Lock()
	defer mu.Unlock()

	var tempBans []map[string]interface{}

	now := time.Now()
	expired := 0

	for pubkey, entry := range tempBannedPubkeys {
		// Check if the temp ban is still active
		if now.Before(entry.unbanTime) {
			tempBans = append(tempBans, map[string]interface{}{
				"pubkey":     pubkey,
				"expires_at": entry.unbanTime.Unix(), // Convert expiration time to Unix timestamp
			})
		} else {
			// If the ban has expired, configLog() and remove it
			// Removing expired temp ban
			configLog().Info("Removing expired temp ban", "pubkey", pubkey)
			delete(tempBannedPubkeys, pubkey)
		}
	}

	if expired > 0 {
		configLog().Debug("Cleaned up expired temporary bans", "count", expired)
	}

	return tempBans
}

func isPubKeyPermanentlyBlacklisted(pubKey string, blacklistConfig *types.BlacklistConfig) bool {
	if blacklistConfig == nil || !blacklistConfig.Enabled {
		return false
	}

	// Check pubkeys.
	for _, blacklistedKey := range blacklistConfig.PermanentBlacklistPubkeys {
		if pubKey == blacklistedKey {
			return true
		}
	}

	// Check npubs.
	for _, npub := range blacklistConfig.PermanentBlacklistNpubs {
		decodedPubKey, err := utils.DecodeNpub(npub)
		if err != nil {
			configLog().Error("Error decoding npub", "npub", npub, "error", err)
			continue
		}
		if pubKey == decodedPubKey {
			return true
		}
	}

	return false
}

func AddToPermanentBlacklist(pubkey string) error {
	blacklistConfig := GetBlacklistConfig()
	if blacklistConfig == nil {
		return fmt.Errorf("blacklist configuration is not loaded")
	}

	// Check if already blacklisted.
	if isPubKeyPermanentlyBlacklisted(pubkey, blacklistConfig) {
		configLog().Debug("Pubkey already in permanent blacklist", "pubkey", pubkey)
		return fmt.Errorf("pubkey %s is already in the permanent blacklist", pubkey)
	}

	// Add pubkey to the permanent blacklist.
	blacklistConfig.PermanentBlacklistPubkeys = append(blacklistConfig.PermanentBlacklistPubkeys, pubkey)

	configLog().Info("Added pubkey to permanent blacklist", "pubkey", pubkey)

	// Persist changes to blacklist.yml.
	err := saveBlacklistConfig(*blacklistConfig)
	if err != nil {
		configLog().Error("Failed to save blacklist configuration", "error", err)
		return err
	}

	configLog().Debug("Saved blacklist configuration to file")
	return nil
}

func saveBlacklistConfig(blacklistConfig types.BlacklistConfig) error {
	data, err := yaml.Marshal(blacklistConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal blacklist config: %v", err)
	}

	err = os.WriteFile("blacklist.yml", data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config to file: %v", err)
	}

	return nil
}

// FetchPubkeysFromLocalMuteList sends a REQ to the local relay for mute list events.
func FetchPubkeysFromLocalMuteList(localRelayURL string, muteListAuthors []string) ([]string, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allPubkeys []string
	results := make(chan []string, 1)

	// Parse WebSocket URL
	wsURL, err := url.Parse(localRelayURL)
	if err != nil {
		// Invalid WebSocket URL
		configLog().Error("Invalid WebSocket URL", "url", localRelayURL, "error", err)
		return nil, err
	}

	// Construct WebSocket origin (required by `x/net/websocket`)
	origin := "http://" + wsURL.Host

	wg.Add(1)
	go func() {
		defer wg.Done()

		// Dial WebSocket connection
		conn, err := websocket.Dial(localRelayURL, "", origin)
		if err != nil {
			// Failed to connect to the local relay
			configLog().Error("Failed to connect to local relay", "url", localRelayURL, "error", err)
			return
		}
		defer conn.Close()

		subscriptionID := "mutelist-fetch"

		// Create the REQ message to fetch the mute list events by IDs.
		req := []interface{}{"REQ", subscriptionID, map[string]interface{}{
			"authors": muteListAuthors,
			"kinds":   []int{10000}, // Mute list events kind.
		}}

		reqJSON, err := json.Marshal(req)
		if err != nil {
			// Failed to marshal WebSocket request
			configLog().Error("Failed to marshal request", "error", err)
			return
		}

		configLog().Debug("Fetching mutelist from local relay", 
			"url", localRelayURL,
			"authors", len(muteListAuthors))

		// Send the message
		if _, err := conn.Write(reqJSON); err != nil {
			// Failed to send request to the local relay
			configLog().Error("Failed to send request to local relay", "url", localRelayURL, "error", err)
			return
		}

		// Listen for messages
		for {
			message := make([]byte, 4096)
			n, err := conn.Read(message)
			if err != nil {
				if err == io.EOF {
					// Connection closed by the server
					configLog().Debug("Connection closed by server")
					break
				}
				// Error reading message from relay
				configLog().Error("Error reading from local relay", "url", localRelayURL, "error", err)

				break
			}

			// Trim message to actual length
			message = message[:n]
			// Received raw WebSocket message
			configLog().Debug("Received WebSocket message", "size", n)

			var response []interface{}
			err = json.Unmarshal(message, &response)
			if err != nil || len(response) < 2 {
				// Invalid WebSocket message format
				configLog().Error("Invalid message format", "error", err)
				continue
			}

			if len(response) > 0 {
				eventType, ok := response[0].(string)
				if !ok {
					configLog().Warn("Unexpected event type", "type", response[0])
					continue
				}

				// Handle "EVENT"
				if eventType == "EVENT" && len(response) >= 3 {
					eventData, ok := response[2].(map[string]interface{})
					if !ok {
						configLog().Warn("Unexpected event data format", "data", response[2])
						continue
					}

					pubkeys := extractPubkeysFromMuteListEvent(eventData)
					configLog().Debug("Extracted pubkeys from mutelist event", "count", len(pubkeys))
					results <- pubkeys
				}

				// Handle "EOSE"
				if eventType == "EOSE" {
					closeReq := []interface{}{"CLOSE", subscriptionID}
					closeReqJSON, _ := json.Marshal(closeReq)
					_, _ = conn.Write(closeReqJSON)
					configLog().Debug("Sent CLOSE request to end subscription")
					break
				}

			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for pubkeys := range results {
		mu.Lock()
		allPubkeys = append(allPubkeys, pubkeys...)
		mu.Unlock()
	}

	configLog().Debug("Total pubkeys fetched from mutelist", "count", len(allPubkeys))
	return allPubkeys, nil
}

// extractPubkeysFromMuteListEvent extracts pubkeys from a mute list event.
func extractPubkeysFromMuteListEvent(eventData map[string]interface{}) []string {
	var pubkeys []string

	tags, ok := eventData["tags"].([]interface{})
	if !ok {
		configLog().Warn("Tags field missing or invalid in mutelist event")
		return pubkeys
	}

	for _, tag := range tags {
		tagArray, ok := tag.([]interface{})
		if ok && len(tagArray) > 1 && tagArray[0] == "p" {
			pubkey, ok := tagArray[1].(string)
			if ok {
				pubkeys = append(pubkeys, pubkey)
			}
		}
	}

	// Extracted pubkeys from mute list event
	configLog().Debug("Extracted pubkeys from mute list event", "count", len(pubkeys))
	return pubkeys
}
