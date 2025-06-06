package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
	"golang.org/x/net/websocket"
)

// Global session manager instance
var SessionMgr *SessionManager

// Application relays for initial discovery
var appRelays []string

// LoginHandler handles user login and session initialization
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	log.Util().Debug("Login handler called")

	if SessionMgr == nil {
		log.Util().Error("SessionMgr not initialized")
		http.Error(w, "Session manager not available", http.StatusInternalServerError)
		return
	}

	// Check if user is already logged in
	if session := SessionMgr.GetCurrentUser(r); session != nil {
		log.Util().Info("User already logged in", "pubkey", session.PublicKey)
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		log.Util().Error("Failed to parse form", "error", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	publicKey := r.FormValue("publicKey")
	if publicKey == "" {
		log.Util().Warn("Missing publicKey in form data")
		http.Error(w, "Missing publicKey", http.StatusBadRequest)
		return
	}
	log.Util().Info("Processing login", "pubkey", publicKey)

	// Try cached data first
	if cachedData, exists := cache.GetUserData(publicKey); exists {
		log.Util().Debug("Found cached user data", "pubkey", publicKey)
		
		// Validate cached data before using
		if isValidCachedData(cachedData) {
			if err := createSessionFromCache(w, publicKey, cachedData); err != nil {
				log.Util().Error("Failed to create session from cache", "pubkey", publicKey, "error", err)
				// Fall through to fetch fresh data
			} else {
				log.Util().Info("Login successful using cached data", "pubkey", publicKey)
				return
			}
		} else {
			log.Util().Warn("Cached data is invalid, clearing cache", "pubkey", publicKey)
			cache.ClearUserData(publicKey)
		}
	}

	// Fetch fresh data from relays
	if err := fetchAndCacheUserData(publicKey); err != nil {
		log.Util().Error("Failed to fetch user data", "pubkey", publicKey, "error", err)
		http.Error(w, "User not found or unreachable", http.StatusNotFound)
		return
	}

	// Create session with fresh data
	if _, err := SessionMgr.CreateSession(w, publicKey); err != nil {
		log.Util().Error("Failed to create session", "pubkey", publicKey, "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	log.Util().Info("Login successful with fresh data", "pubkey", publicKey)
}

// isValidCachedData checks if cached data contains valid user information
func isValidCachedData(cachedData cache.CachedUserData) bool {
	if cachedData.Metadata == "" {
		return false
	}
	
	// Try to parse metadata to ensure it's valid JSON
	var metadata nostr.Event
	if err := json.Unmarshal([]byte(cachedData.Metadata), &metadata); err != nil {
		return false
	}
	
	// Basic validation - must have ID and PubKey
	return metadata.ID != "" && metadata.PubKey != ""
}

// createSessionFromCache creates a session using cached user data
func createSessionFromCache(w http.ResponseWriter, publicKey string, cachedData cache.CachedUserData) error {
	// Parse cached metadata to verify it's still valid
	var metadata nostr.Event
	if err := json.Unmarshal([]byte(cachedData.Metadata), &metadata); err != nil {
		return fmt.Errorf("invalid cached metadata: %w", err)
	}
	
	// Verify the cached metadata matches the requested public key
	if metadata.PubKey != publicKey {
		return fmt.Errorf("cached metadata pubkey mismatch")
	}
	
	// Create session
	session, err := SessionMgr.CreateSession(w, publicKey)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	
	log.Util().Debug("Session created from cache", "pubkey", session.PublicKey)
	return nil
}

// fetchAndCacheUserData fetches and caches user data without maintaining connections
func fetchAndCacheUserData(publicKey string) error {
	log.Util().Debug("Fetching fresh user data", "pubkey", publicKey)

	// Fetch mailboxes from initial app relays
	mailboxes, err := fetchUserMailboxes(publicKey, appRelays)
	if err != nil {
		log.Util().Warn("Failed to fetch mailboxes", "pubkey", publicKey, "error", err)
	}

	// Determine which relays to use for metadata
	var relaysForMetadata []string
	if mailboxes != nil {
		relaysForMetadata = mailboxes.ToStringSlice()
	}
	
	// Use initial app relays as fallback
	if len(relaysForMetadata) == 0 {
		relaysForMetadata = appRelays
		log.Util().Info("Using app relays for metadata", "pubkey", publicKey, "relay_count", len(relaysForMetadata))
	} else {
		log.Util().Info("Using user mailboxes for metadata", "pubkey", publicKey, "relay_count", len(relaysForMetadata))
	}

	// Fetch metadata
	userMetadata, err := fetchUserMetadata(publicKey, relaysForMetadata)
	if err != nil || userMetadata == nil {
		return fmt.Errorf("failed to fetch user metadata: %w", err)
	}

	// Cache the data
	cacheUserData(publicKey, userMetadata, mailboxes)

	log.Util().Info("User data fetched and cached successfully", "pubkey", publicKey)
	return nil
}

// RebuildCacheForSession rebuilds cache for an existing session (useful after app restart)
func RebuildCacheForSession(session *UserSession) {
	log.Util().Info("Rebuilding cache for existing session", "pubkey", session.PublicKey)
	
	go func() {
		// Fetch data in background to avoid blocking the request
		if err := fetchAndCacheUserData(session.PublicKey); err != nil {
			log.Util().Error("Failed to rebuild cache for session", 
				"pubkey", session.PublicKey, "error", err)
		} else {
			log.Util().Info("Cache rebuilt successfully for session", 
				"pubkey", session.PublicKey)
		}
	}()
}

// GetUserProfile retrieves user profile data (metadata + mailboxes) with cache fallback
func GetUserProfile(publicKey string) (metadata *nostr.Event, mailboxes *core.Mailboxes, err error) {
	// Try cache first
	if cachedData, exists := cache.GetUserData(publicKey); exists && isValidCachedData(cachedData) {
		if err := json.Unmarshal([]byte(cachedData.Metadata), &metadata); err == nil {
			// Parse mailboxes if available
			if cachedData.Mailboxes != "" && cachedData.Mailboxes != "{}" {
				json.Unmarshal([]byte(cachedData.Mailboxes), &mailboxes)
			}
			log.Util().Debug("Retrieved profile from cache", "pubkey", publicKey)
			return metadata, mailboxes, nil
		}
	}
	
	// Fetch fresh data
	log.Util().Debug("Cache miss, fetching fresh profile data", "pubkey", publicKey)
	
	mailboxes, _ = fetchUserMailboxes(publicKey, appRelays)
	
	relaysForMetadata := appRelays
	if mailboxes != nil {
		relaysForMetadata = mailboxes.ToStringSlice()
	}
	
	metadata, err = fetchUserMetadata(publicKey, relaysForMetadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch profile: %w", err)
	}
	
	// Cache the fresh data
	cacheUserData(publicKey, metadata, mailboxes)
	
	return metadata, mailboxes, nil
}

// fetchUserMailboxes fetches mailboxes using direct WebSocket connections
func fetchUserMailboxes(publicKey string, relays []string) (*core.Mailboxes, error) {
	log.Util().Debug("Fetching user mailboxes", "pubkey", publicKey, "relay_count", len(relays))

	for _, relayURL := range relays {
		mailbox, err := fetchMailboxFromRelay(publicKey, relayURL)
		if err != nil {
			log.Util().Warn("Failed to fetch mailbox from relay", "relay", relayURL, "error", err)
			continue
		}
		
		if mailbox != nil && (len(mailbox.Read) > 0 || len(mailbox.Write) > 0 || len(mailbox.Both) > 0) {
			log.Util().Debug("Found mailboxes", "relay", relayURL, 
				"read_count", len(mailbox.Read),
				"write_count", len(mailbox.Write), 
				"both_count", len(mailbox.Both))
			return mailbox, nil
		}
	}
	
	return nil, nil
}

// fetchUserMetadata fetches metadata using direct WebSocket connections
func fetchUserMetadata(publicKey string, relays []string) (*nostr.Event, error) {
	log.Util().Debug("Fetching user metadata", "pubkey", publicKey, "relay_count", len(relays))

	for _, relayURL := range relays {
		metadata, err := fetchMetadataFromRelay(publicKey, relayURL)
		if err != nil {
			log.Util().Warn("Failed to fetch metadata from relay", "relay", relayURL, "error", err)
			continue
		}
		
		if metadata != nil {
			log.Util().Debug("Found metadata", "relay", relayURL, "event_id", metadata.ID)
			return metadata, nil
		}
	}
	
	return nil, fmt.Errorf("no metadata found")
}

// fetchMailboxFromRelay fetches mailboxes from a single relay using direct WebSocket
func fetchMailboxFromRelay(publicKey string, relayURL string) (*core.Mailboxes, error) {
	log.Util().Debug("Connecting to relay for mailboxes", "relay", relayURL)

	origin := "http://localhost/"
	conn, err := websocket.Dial(relayURL, "", origin)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	subscriptionID := "mailbox-sub"
	
	filter := map[string]interface{}{
		"authors": []string{publicKey},
		"kinds":   []int{10002},
	}

	subRequest := []interface{}{"REQ", subscriptionID, filter}
	requestJSON, err := json.Marshal(subRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Util().Debug("Sending subscription request", "relay", relayURL)
	if err := websocket.Message.Send(conn, string(requestJSON)); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	var mailbox *core.Mailboxes

	for {
		var messageStr string
		if err := websocket.Message.Receive(conn, &messageStr); err != nil {
			return mailbox, nil // Timeout or connection closed
		}

		var response []interface{}
		if err := json.Unmarshal([]byte(messageStr), &response); err != nil {
			log.Util().Warn("Failed to parse response", "relay", relayURL, "error", err)
			continue
		}

		switch response[0] {
		case "EVENT":
			if len(response) < 3 {
				continue
			}

			var event nostr.Event
			eventData, _ := json.Marshal(response[2])
			if err := json.Unmarshal(eventData, &event); err != nil {
				log.Util().Warn("Failed to parse event", "relay", relayURL, "error", err)
				continue
			}

			log.Util().Debug("Received mailbox event", "relay", relayURL, "event_id", event.ID)

			if mailbox == nil {
				mailbox = &core.Mailboxes{}
			}

			// Parse relay tags
			for _, tag := range event.Tags {
				if len(tag) >= 2 && tag[0] == "r" {
					relayURL := tag[1]
					if len(tag) >= 3 {
						switch tag[2] {
						case "read":
							mailbox.Read = append(mailbox.Read, relayURL)
						case "write":
							mailbox.Write = append(mailbox.Write, relayURL)
						}
					} else {
						mailbox.Both = append(mailbox.Both, relayURL)
					}
				}
			}

		case "EOSE":
			log.Util().Debug("Received EOSE", "relay", relayURL)
			
			closeRequest := []interface{}{"CLOSE", subscriptionID}
			closeJSON, _ := json.Marshal(closeRequest)
			websocket.Message.Send(conn, string(closeJSON))
			
			return mailbox, nil

		case "NOTICE":
			if len(response) > 1 {
				notice, _ := response[1].(string)
				log.Util().Debug("Relay notice", "relay", relayURL, "notice", notice)
			}
		}
	}
}

// fetchMetadataFromRelay fetches metadata from a single relay using direct WebSocket
func fetchMetadataFromRelay(publicKey string, relayURL string) (*nostr.Event, error) {
	log.Util().Debug("Connecting to relay for metadata", "relay", relayURL)

	origin := "http://localhost/"
	conn, err := websocket.Dial(relayURL, "", origin)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	subscriptionID := "metadata-sub"
	
	filter := map[string]interface{}{
		"authors": []string{publicKey},
		"kinds":   []int{0},
		"limit":   1,
	}

	subRequest := []interface{}{"REQ", subscriptionID, filter}
	requestJSON, err := json.Marshal(subRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Util().Debug("Sending subscription request", "relay", relayURL)
	if err := websocket.Message.Send(conn, string(requestJSON)); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	var latestMetadata *nostr.Event

	for {
		var messageStr string
		if err := websocket.Message.Receive(conn, &messageStr); err != nil {
			return latestMetadata, nil // Timeout or connection closed
		}

		var response []interface{}
		if err := json.Unmarshal([]byte(messageStr), &response); err != nil {
			log.Util().Warn("Failed to parse response", "relay", relayURL, "error", err)
			continue
		}

		switch response[0] {
		case "EVENT":
			if len(response) < 3 {
				continue
			}

			var event nostr.Event
			eventData, _ := json.Marshal(response[2])
			if err := json.Unmarshal(eventData, &event); err != nil {
				log.Util().Warn("Failed to parse event", "relay", relayURL, "error", err)
				continue
			}

			log.Util().Debug("Received metadata event", "relay", relayURL, "event_id", event.ID)

			if latestMetadata == nil || event.CreatedAt > latestMetadata.CreatedAt {
				latestMetadata = &event
			}

		case "EOSE":
			log.Util().Debug("Received EOSE", "relay", relayURL)
			
			closeRequest := []interface{}{"CLOSE", subscriptionID}
			closeJSON, _ := json.Marshal(closeRequest)
			websocket.Message.Send(conn, string(closeJSON))
			
			return latestMetadata, nil

		case "NOTICE":
			if len(response) > 1 {
				notice, _ := response[1].(string)
				log.Util().Debug("Relay notice", "relay", relayURL, "notice", notice)
			}
		}
	}
}

// cacheUserData caches user metadata and mailboxes
func cacheUserData(publicKey string, metadata *nostr.Event, mailboxes *core.Mailboxes) {
	mailboxesJSON := "{}"
	if mailboxes != nil {
		if data, err := json.Marshal(mailboxes); err == nil {
			mailboxesJSON = string(data)
		}
	}

	if metadataJSON, err := json.Marshal(metadata); err == nil {
		cache.SetUserData(publicKey, string(metadataJSON), mailboxesJSON)
		log.Util().Debug("User data cached successfully", "pubkey", publicKey)
	}
}

// SetAppRelays initializes the application relays for initial discovery
func SetAppRelays(relays []string) {
	appRelays = relays
	log.Util().Debug("App relays initialized for discovery", "relay_count", len(relays))
}