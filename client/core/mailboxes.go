package core

import (
	"encoding/json"
	"time"

	"github.com/0ceanslim/grain/client/core/helpers"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Mailboxes represents a user's relay preferences from NIP-65
type Mailboxes struct {
	Read  []string `json:"read"`
	Write []string `json:"write"`
	Both  []string `json:"both"`
}

// ToStringSlice combines Read, Write, and Both into a single []string
func (m Mailboxes) ToStringSlice() []string {
	var urls []string
	urls = append(urls, m.Read...)
	urls = append(urls, m.Write...)
	urls = append(urls, m.Both...)
	return urls
}

// FetchUserMailboxes fetches a user's relay list (kind 10002) from available relays
func FetchUserMailboxes(publicKey string, relays []string) (*Mailboxes, error) {
	var mailboxes Mailboxes
	
	log.Util().Debug("Fetching user mailboxes", 
		"pubkey", publicKey, 
		"relay_count", len(relays))

	for _, url := range relays {
		log.Util().Debug("Connecting to relay for mailboxes", "relay", url)

		conn, err := helpers.DialWithTimeout(url, 5*time.Second)
		if err != nil {
			log.Util().Warn("Failed to connect to relay", "relay", url, "error", err)
			continue
		}

		subscriptionID := "sub-mailboxes"

		filter := nostr.Filter{
			Authors: []string{publicKey},
			Kinds:   []int{10002}, // NIP-65 relay list metadata
		}

		subRequest := []interface{}{"REQ", subscriptionID, filter}
		requestJSON, err := json.Marshal(subRequest)
		if err != nil {
			log.Util().Error("Failed to marshal subscription request", "error", err)
			conn.Close()
			return nil, err
		}

		log.Util().Debug("Sending mailboxes subscription request", "relay", url)
		if _, err := conn.Write(requestJSON); err != nil {
			log.Util().Warn("Failed to send subscription request", "relay", url, "error", err)
			conn.Close()
			continue
		}

		// Read messages until EOSE
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

				log.Util().Debug("Received mailboxes event", "relay", url, "event_id", event.ID)

				// Parse relay tags
				for _, tag := range event.Tags {
					if len(tag) > 1 && tag[0] == "r" {
						relayURL := tag[1]
						if len(tag) == 3 {
							switch tag[2] {
							case "read":
								mailboxes.Read = append(mailboxes.Read, relayURL)
							case "write":
								mailboxes.Write = append(mailboxes.Write, relayURL)
							}
						} else {
							// No marker means both read and write
							mailboxes.Both = append(mailboxes.Both, relayURL)
						}
					}
				}

			case "EOSE":
				log.Util().Debug("Received EOSE for mailboxes", "relay", url)
				
				// Send CLOSE and cleanup
				helpers.SendCloseMessage(conn, subscriptionID)
				conn.Close()
				
				// Return mailboxes if we found any
				if len(mailboxes.Read) > 0 || len(mailboxes.Write) > 0 || len(mailboxes.Both) > 0 {
					log.Util().Info("Successfully fetched user mailboxes", 
						"pubkey", publicKey,
						"read_count", len(mailboxes.Read),
						"write_count", len(mailboxes.Write), 
						"both_count", len(mailboxes.Both))
					return &mailboxes, nil
				}
				
				// No mailboxes found on this relay, try next
				break
			}
		}
		
		conn.Close()
	}

	// No mailboxes found on any relay
	if len(mailboxes.Read) == 0 && len(mailboxes.Write) == 0 && len(mailboxes.Both) == 0 {
		log.Util().Info("No mailboxes found for user", "pubkey", publicKey)
		return nil, nil
	}

	return &mailboxes, nil
}