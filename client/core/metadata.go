package core

import (
	"encoding/json"
	"time"

	"github.com/0ceanslim/grain/client/core/helpers"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// FetchUserMetadata fetches a user's metadata (kind 0) from available relays
func FetchUserMetadata(publicKey string, relays []string) (*nostr.Event, error) {
	log.Util().Debug("Fetching user metadata", 
		"pubkey", publicKey, 
		"relay_count", len(relays))

	for _, url := range relays {
		log.Util().Debug("Connecting to relay for metadata", "relay", url)

		conn, err := helpers.DialWithTimeout(url, 5*time.Second)
		if err != nil {
			log.Util().Warn("Failed to connect to relay", "relay", url, "error", err)
			continue
		}

		subscriptionID := "sub-metadata"

		filter := nostr.Filter{
			Authors: []string{publicKey},
			Kinds:   []int{0}, // NIP-01 user metadata
		}

		subRequest := []interface{}{"REQ", subscriptionID, filter}
		requestJSON, err := json.Marshal(subRequest)
		if err != nil {
			log.Util().Error("Failed to marshal subscription request", "error", err)
			conn.Close()
			return nil, err
		}

		log.Util().Debug("Sending metadata subscription request", "relay", url)
		if _, err := conn.Write(requestJSON); err != nil {
			log.Util().Warn("Failed to send subscription request", "relay", url, "error", err)
			conn.Close()
			continue
		}

		// Read messages until EOSE or we find metadata
		var latestMetadata *nostr.Event
		
		for {
			message, err := helpers.ReadMessageWithTimeout(conn, 10*time.Second)
			if err != nil {
				log.Util().Warn("Error reading from relay", "relay", url, "error", err)
				break
			}

			var response []interface{}
			if err := json.Unmarshal(message, &response); err != nil {
				log.Util().Warn("Failed to parse response", "relay", url, "error", err)
				continue
			}

			switch response[0] {
			case "EVENT":
				var event nostr.Event
				eventData, _ := json.Marshal(response[2])
				if err := json.Unmarshal(eventData, &event); err != nil {
					log.Util().Warn("Failed to parse event", "relay", url, "error", err)
					continue
				}

				log.Util().Debug("Received metadata event", 
					"relay", url, 
					"event_id", event.ID,
					"created_at", event.CreatedAt)

				// Keep the most recent metadata event
				if latestMetadata == nil || event.CreatedAt > latestMetadata.CreatedAt {
					latestMetadata = &event
				}

			case "EOSE":
				log.Util().Debug("Received EOSE for metadata", "relay", url)
				
				// Send CLOSE and cleanup
				helpers.SendCloseMessage(conn, subscriptionID)
				conn.Close()
				
				// Return metadata if we found any
				if latestMetadata != nil {
					log.Util().Info("Successfully fetched user metadata", 
						"pubkey", publicKey,
						"event_id", latestMetadata.ID,
						"relay", url)
					return latestMetadata, nil
				}
				
				// No metadata found on this relay, try next
				break
			}
		}
		
		conn.Close()
	}

	// No metadata found on any relay
	log.Util().Info("No metadata found for user", "pubkey", publicKey)
	return nil, nil
}