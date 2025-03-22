package config

import (
	"encoding/json"
	"fmt"
	"io"

	//"log"
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

// CheckBlacklist checks if a pubkey is in the blacklist based on event content
func CheckBlacklist(pubkey, eventContent string) (bool, string) {
	blacklistConfig := GetBlacklistConfig()
	if blacklistConfig == nil || !blacklistConfig.Enabled {
		return false, ""
	}

	// Checking the blacklist for a pubkey
	log.Info(fmt.Sprintf("Checking blacklist for pubkey: %s", pubkey))

	// Check for permanent blacklist by pubkey or npub.
	if isPubKeyPermanentlyBlacklisted(pubkey, blacklistConfig) {
		// Permanent blacklist match
		log.Info(fmt.Sprintf("Pubkey %s is permanently blacklisted", pubkey))
		return true, fmt.Sprintf("pubkey %s is permanently blacklisted", pubkey)
	}

	// Check for temporary ban.
	if isPubKeyTemporarilyBlacklisted(pubkey) {
		// Temporary blacklist match
		log.Info(fmt.Sprintf("Pubkey %s is temporarily blacklisted", pubkey))
		return true, fmt.Sprintf("pubkey %s is temporarily blacklisted", pubkey)
	}

	// Check for permanent ban based on wordlist
	for _, word := range blacklistConfig.PermanentBanWords {
		if strings.Contains(eventContent, word) {
			err := AddToPermanentBlacklist(pubkey)
			if err != nil {
				log.Error("Failed to add pubkey to permanent blacklist", "pubkey", pubkey, "error", err)
				return true, fmt.Sprintf("pubkey %s is permanently banned and failed to save: %v", pubkey, err)
			}
			log.Info("Pubkey permanently banned due to wordlist match", "pubkey", pubkey, "word", word)
			return true, "blocked: pubkey is permanently banned"
		}
	}

	// Check for temporary ban based on wordlist
	for _, word := range blacklistConfig.TempBanWords {
		if strings.Contains(eventContent, word) {
			err := AddToTemporaryBlacklist(pubkey, *blacklistConfig)
			if err != nil {
				log.Error("Failed to add pubkey to temporary blacklist", "pubkey", pubkey, "error", err)
				return true, fmt.Sprintf("pubkey %s is temporarily banned and failed to save: %v", pubkey, err)
			}
			log.Info("Pubkey temporarily banned due to wordlist match", "pubkey", pubkey, "word", word)
			return true, "blocked: pubkey is temporarily banned"
		}
	}

	// Check mutelist blacklist
	if len(blacklistConfig.MuteListAuthors) > 0 {
		cfg := GetConfig()
		if cfg == nil {
			return true, "Internal server error: server configuration is missing"
		}

		localRelayURL := fmt.Sprintf("ws://localhost%s", cfg.Server.Port)
		mutelistedPubkeys, err := FetchPubkeysFromLocalMuteList(localRelayURL, blacklistConfig.MuteListAuthors)
		if err != nil {
			// Error fetching pubkeys from mute list
			log.Error("Error fetching pubkeys from mutelist", "error", err)
			return true, "Error fetching pubkeys from mutelist"
		}

		for _, mutelistedPubkey := range mutelistedPubkeys {
			if pubkey == mutelistedPubkey {
				// Pubkey found in the mutelist
				log.Info(fmt.Sprintf("Pubkey %s is in the mutelist", pubkey))
				return true, "not allowed: pubkey is in mutelist"
			}
		}
	} else {
		// No mutelist event IDs specified
		log.Info("No mutelist event IDs specified in the blacklist configuration")
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
		log.Debug(fmt.Sprintf("Pubkey %s not found in temporary blacklist", pubkey))
		return false
	}

	now := time.Now()
	if now.After(entry.unbanTime) {
		// Temporary ban expired
		log.Info(fmt.Sprintf("Temporary ban for pubkey %s has expired. Count: %d", pubkey, entry.count))
		return false
	}

	// Pubkey currently blacklisted
	log.Warn(fmt.Sprintf("Pubkey %s is currently temporarily blacklisted. Count: %d, Unban time: %s", pubkey, entry.count, entry.unbanTime))
	return true
}

func ClearTemporaryBans() {
	mu.Lock()
	defer mu.Unlock()
	tempBannedPubkeys = make(map[string]*tempBanEntry)
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
		log.Info(fmt.Sprintf("Creating new temporary ban entry for pubkey %s", pubkey))
		entry = &tempBanEntry{
			count:     0,
			unbanTime: time.Now(),
		}
		tempBannedPubkeys[pubkey] = entry
	} else {
		// Updating an existing temp ban entry
		log.Info(fmt.Sprintf("Updating existing temporary ban entry for pubkey %s. Current count: %d", pubkey, entry.count))

		if time.Now().After(entry.unbanTime) {
			// Previous ban expired, keeping count
			log.Info(fmt.Sprintf("Previous ban for pubkey %s has expired. Keeping count at %d", pubkey, entry.count))

		}
	}

	// Increment the count
	entry.count++
	entry.unbanTime = time.Now().Add(time.Duration(blacklistConfig.TempBanDuration) * time.Second)

	// Updating temp ban count
	log.Info(fmt.Sprintf("Pubkey %s temporary ban count updated to: %d, MaxTempBans: %d, New unban time: %s", pubkey, entry.count, blacklistConfig.MaxTempBans, entry.unbanTime))

	if entry.count > blacklistConfig.MaxTempBans {
		// Attempting to move to permanent blacklist
		log.Warn(fmt.Sprintf("Attempting to move pubkey %s to permanent blacklist", pubkey))

		delete(tempBannedPubkeys, pubkey)

		// Release the lock before calling AddToPermanentBlacklist
		mu.Unlock()
		err := AddToPermanentBlacklist(pubkey)
		mu.Lock() // Re-acquire the lock

		if err != nil {
			// Error adding to permanent blacklist
			log.Error("Error adding pubkey to permanent blacklist", "pubkey", pubkey, "error", err)
			return err
		}
		// Successfully added to permanent blacklist
		log.Info(fmt.Sprintf("Successfully added pubkey %s to permanent blacklist", pubkey))
	}

	return nil
}

// GetTemporaryBlacklist fetches all currently active temporary bans
func GetTemporaryBlacklist() []map[string]interface{} {
	mu.Lock()
	defer mu.Unlock()

	var tempBans []map[string]interface{}

	now := time.Now()

	for pubkey, entry := range tempBannedPubkeys {
		// Check if the temp ban is still active
		if now.Before(entry.unbanTime) {
			tempBans = append(tempBans, map[string]interface{}{
				"pubkey":     pubkey,
				"expires_at": entry.unbanTime.Unix(), // Convert expiration time to Unix timestamp
			})
		} else {
			// If the ban has expired, log and remove it
			// Removing expired temp ban
			log.Info(fmt.Sprintf("Removing expired temp ban for pubkey: %s", pubkey))
			delete(tempBannedPubkeys, pubkey)
		}
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
	blacklistConfig := GetBlacklistConfig()
	if blacklistConfig == nil {
		return fmt.Errorf("blacklist configuration is not loaded")
	}

	// Check if already blacklisted.
	if isPubKeyPermanentlyBlacklisted(pubkey, blacklistConfig) {
		return fmt.Errorf("pubkey %s is already in the permanent blacklist", pubkey)
	}

	// Add pubkey to the permanent blacklist.
	blacklistConfig.PermanentBlacklistPubkeys = append(blacklistConfig.PermanentBlacklistPubkeys, pubkey)

	// Persist changes to blacklist.yml.
	return saveBlacklistConfig(*blacklistConfig)
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
		log.Error("Invalid WebSocket URL", "url", localRelayURL, "error", err)
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
			log.Error("Failed to connect to local relay", "relay_url", localRelayURL, "error", err)
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
			log.Error("Failed to marshal request", "error", err)
			return
		}

		// Send the message
		if _, err := conn.Write(reqJSON); err != nil {
			// Failed to send request to the local relay
			log.Error("Failed to send request to local relay", "relay_url", localRelayURL, "error", err)
			return
		}

		// Listen for messages
		for {
			message := make([]byte, 4096)
			n, err := conn.Read(message)
			if err != nil {
				if err == io.EOF {
					// Connection closed by the server
					log.Warn("Connection closed by the server (EOF)")
					break
				}
				// Error reading message from relay
				log.Error("Error reading message from local relay", "relay_url", localRelayURL, "error", err)

				break
			}

			// Trim message to actual length
			message = message[:n]
			// Received raw WebSocket message
			log.Debug(fmt.Sprintf("Received raw message: %s", message))

			var response []interface{}
			err = json.Unmarshal(message, &response)
			if err != nil || len(response) < 2 {
				// Invalid WebSocket message format
				log.Error("Invalid message format or failed to unmarshal", "error", err)
				continue
			}

			if len(response) > 0 {
				eventType, ok := response[0].(string)
				if !ok {
					log.Warn("Unexpected event type", "type", response[0])
					continue
				}

				// Handle "EVENT"
				if eventType == "EVENT" && len(response) >= 3 {
					eventData, ok := response[2].(map[string]interface{})
					if !ok {
						log.Warn("Unexpected event data format", "data", response[2])
						continue
					}

					pubkeys := extractPubkeysFromMuteListEvent(eventData)
					results <- pubkeys
				}

				// Handle "EOSE"
				if eventType == "EOSE" {
					closeReq := []interface{}{"CLOSE", subscriptionID}
					closeReqJSON, _ := json.Marshal(closeReq)
					_, _ = conn.Write(closeReqJSON)
					log.Info("Sent CLOSE request to end subscription.")
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

	return allPubkeys, nil
}

// extractPubkeysFromMuteListEvent extracts pubkeys from a mute list event.
func extractPubkeysFromMuteListEvent(eventData map[string]interface{}) []string {
	var pubkeys []string

	tags, ok := eventData["tags"].([]interface{})
	if !ok {
		log.Warn("Tags field is missing or not an array")
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
	log.Debug(fmt.Sprintf("Extracted pubkeys: %v", pubkeys))
	return pubkeys
}
