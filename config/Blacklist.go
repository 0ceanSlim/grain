package config

import (
	"encoding/json"
	"fmt"
	types "grain/config/types"
	"grain/server/utils"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"
)

// CheckBlacklist checks if a pubkey is in the blacklist based on event content
func CheckBlacklist(pubkey, eventContent string) (bool, string) {
	blacklistConfig := GetBlacklistConfig()
	if blacklistConfig == nil || !blacklistConfig.Enabled {
		return false, ""
	}

	log.Printf("Checking blacklist for pubkey: %s", pubkey)

	// Check for permanent blacklist by pubkey or npub.
	if isPubKeyPermanentlyBlacklisted(pubkey, blacklistConfig) {
		log.Printf("Pubkey %s is permanently blacklisted", pubkey)
		return true, fmt.Sprintf("pubkey %s is permanently blacklisted", pubkey)
	}

	// Check for temporary ban.
	if isPubKeyTemporarilyBlacklisted(pubkey) {
		log.Printf("Pubkey %s is temporarily blacklisted", pubkey)
		return true, fmt.Sprintf("pubkey %s is temporarily blacklisted", pubkey)
	}

	// Check for permanent ban based on wordlist.
	for _, word := range blacklistConfig.PermanentBanWords {
		if strings.Contains(eventContent, word) {
			err := AddToPermanentBlacklist(pubkey)
			if err != nil {
				return true, fmt.Sprintf("pubkey %s is permanently banned and failed to save: %v", pubkey, err)
			}
			return true, "blocked: pubkey is permanently banned"
		}
	}

	// Check for temporary ban based on wordlist.
	for _, word := range blacklistConfig.TempBanWords {
		if strings.Contains(eventContent, word) {
			err := AddToTemporaryBlacklist(pubkey, *blacklistConfig)
			if err != nil {
				return true, fmt.Sprintf("pubkey %s is temporarily banned and failed to save: %v", pubkey, err)
			}
			return true, "blocked: pubkey is temporarily banned"
		}
	}

	// Check mutelist blacklist
	if len(blacklistConfig.MuteListAuthors) > 0 {
		cfg := GetConfig()
		if cfg == nil {
			log.Println("Server configuration is not loaded")
			return true, "Internal server error: server configuration is missing"
		}

		localRelayURL := fmt.Sprintf("ws://localhost%s", cfg.Server.Port)
		mutelistedPubkeys, err := FetchPubkeysFromLocalMuteList(localRelayURL, blacklistConfig.MuteListAuthors)
		if err != nil {
			log.Printf("Error fetching pubkeys from mutelist: %v", err)
			return true, "Error fetching pubkeys from mutelist"
		}

		for _, mutelistedPubkey := range mutelistedPubkeys {
			if pubkey == mutelistedPubkey {
				log.Printf("Pubkey %s is in the mutelist", pubkey)
				return true, "not allowed: pubkey is in mutelist"
			}
		}
	} else {
		log.Println("No mutelist event IDs specified in the blacklist configuration")
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
	entry.unbanTime = time.Now().Add(time.Duration(blacklistConfig.TempBanDuration) * time.Second)

	log.Printf("Pubkey %s temporary ban count updated to: %d, MaxTempBans: %d, New unban time: %s", pubkey, entry.count, blacklistConfig.MaxTempBans, entry.unbanTime)

	if entry.count > blacklistConfig.MaxTempBans {
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
			log.Printf("Removing expired temp ban for pubkey: %s", pubkey)
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

	wg.Add(1)
	go func() {
		defer wg.Done()

		conn, _, err := websocket.DefaultDialer.Dial(localRelayURL, nil)
		if err != nil {
			log.Printf("Failed to connect to local relay %s: %v", localRelayURL, err)
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
			log.Printf("Failed to marshal request: %v", err)
			return
		}

		err = conn.WriteMessage(websocket.TextMessage, reqJSON)
		if err != nil {
			log.Printf("Failed to send request to local relay %s: %v", localRelayURL, err)
			return
		}

		// Listen for messages from the local relay.
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Error reading message from local relay %s: %v", localRelayURL, err)
				break
			}

			// Log the raw message for debugging.
			log.Printf("Received raw message: %s", message)

			var response []interface{}
			err = json.Unmarshal(message, &response)
			if err != nil || len(response) < 2 {
				log.Printf("Invalid message format or failed to unmarshal: %v", err)
				continue
			}

			// Check for "EVENT" type messages.
			eventType, ok := response[0].(string)
			if !ok {
				log.Printf("Unexpected event type: %v", response[0])
				continue
			}

			if eventType == "EOSE" {
				// End of subscription events; send a "CLOSE" message to the relay.
				closeReq := []interface{}{"CLOSE", subscriptionID}
				closeReqJSON, err := json.Marshal(closeReq)
				if err != nil {
					log.Printf("Failed to marshal close request: %v", err)
				} else {
					if err = conn.WriteMessage(websocket.TextMessage, closeReqJSON); err != nil {
						log.Printf("Failed to send close request to relay %s: %v", localRelayURL, err)
					} else {
						log.Println("Sent CLOSE request to end subscription.")

						// Wait for a potential response or timeout
						conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
						_, _, err = conn.ReadMessage()
						if err != nil {
							if err == io.EOF {
								log.Println("Connection closed by the server after CLOSE request (EOF)")
							} else if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
								log.Println("WebSocket closed normally after CLOSE request")
							} else {
								log.Printf("Unexpected error after CLOSE request: %v", err)
							}
						}
					}
				}

				// Ensure we break the loop after handling EOSE
				break
			}

			if eventType == "EVENT" {
				// Safely cast the event data from the third element.
				if len(response) < 3 {
					log.Printf("Unexpected event format with insufficient data: %v", response)
					continue
				}

				eventData, ok := response[2].(map[string]interface{})
				if !ok {
					log.Printf("Expected event data to be a map, got: %T", response[2])
					continue
				}

				// Log event data for debugging.
				log.Printf("Event data received: %+v", eventData)

				pubkeys := extractPubkeysFromMuteListEvent(eventData)
				results <- pubkeys
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results from the relay.
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
		log.Println("Tags field is missing or not an array")
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

	log.Printf("Extracted pubkeys: %v", pubkeys)
	return pubkeys
}
