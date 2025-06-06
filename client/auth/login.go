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
)

// Global session manager instance - initialize this in your startup code
var SessionMgr *SessionManager

// Application relays for initial fetching - these are just for discovery
var appRelays []string

// LoginHandler handles user login and session initialization
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	log.Util().Debug("Login handler called")

	// Check if SessionMgr is initialized
	if SessionMgr == nil {
		log.Util().Error("SessionMgr not initialized")
		http.Error(w, "Session manager not available", http.StatusInternalServerError)
		return
	}

	// Check if user is already logged in
	token := SessionMgr.GetSessionToken(r)
	if token != "" {
		session := SessionMgr.GetUserSession(token)
		if session != nil {
			log.Util().Info("User already logged in", "pubkey", session.PublicKey)
			return
		}
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
	log.Util().Info("Received publicKey", "pubkey", publicKey)

	// Try to get user data from cache first
	cachedUserData, exists := cache.GetUserData(publicKey)
	if exists {
		log.Util().Debug("Found cached user data", "pubkey", publicKey)
		
		// Parse cached data
		var userMetadata nostr.Event
		if err := json.Unmarshal([]byte(cachedUserData.Metadata), &userMetadata); err != nil {
			log.Util().Warn("Failed to parse cached metadata", "pubkey", publicKey, "error", err)
		} else {
			var mailboxes core.Mailboxes
			if err := json.Unmarshal([]byte(cachedUserData.Mailboxes), &mailboxes); err != nil {
				log.Util().Warn("Failed to parse cached mailboxes", "pubkey", publicKey, "error", err)
			} else {
				// Create session with cached data
				session, err := SessionMgr.CreateSession(w, publicKey)
				if err == nil {
					// Set up relay connections using cached mailboxes
					allRelays := mailboxes.ToStringSlice()
					if len(allRelays) > 0 {
						SessionMgr.UpdateUserRelays(SessionMgr.GetSessionToken(r), allRelays)
					}
					log.Util().Info("Using cached data and established relay connections", 
						"pubkey", session.PublicKey,
						"relay_count", len(allRelays))
					return
				}
				log.Util().Warn("Failed to create session with cached data, fetching fresh", 
					"pubkey", publicKey, "error", err)
			}
		}
	}

	// Fall back to fetching data from relays using temporary connections
	log.Util().Debug("Fetching user data from discovery relays", "pubkey", publicKey)

	// Use the session-based approach but with app relays for initial discovery
	tempPool := core.NewSessionRelayPool("temp-" + publicKey[:8])
	defer tempPool.DisconnectAll() // Clean up temporary connections

	// Connect to discovery relays
	tempPool.ConnectToAll(appRelays)
	
	// Give connections time to establish
	time.Sleep(2 * time.Second)

	// Fetch user mailboxes using temporary pool
	mailboxes, err := fetchUserMailboxesWithPool(publicKey, appRelays, tempPool)
	if err != nil {
		log.Util().Error("Failed to fetch user relays", "pubkey", publicKey, "error", err)
		http.Error(w, "Failed to fetch user relays", http.StatusInternalServerError)
		return
	}

	// Build relay list - safely handle potential nil
	var allRelays []string
	if mailboxes != nil {
		allRelays = mailboxes.ToStringSlice()
	}

	// If no relays found, use app relays as fallback
	if len(allRelays) == 0 {
		allRelays = appRelays
		log.Util().Info("Using app relays as fallback", "pubkey", publicKey, "relay_count", len(allRelays))
	} else {
		log.Util().Info("Fetched user relays", "pubkey", publicKey, "relay_count", len(allRelays))
	}

	// Fetch metadata using the user's relays or fallback relays
	userMetadata, err := fetchUserMetadataWithPool(publicKey, allRelays, tempPool)
	if err != nil || userMetadata == nil {
		log.Util().Error("Failed to fetch user metadata", "pubkey", publicKey, "error", err)
		http.Error(w, "Failed to fetch user metadata", http.StatusInternalServerError)
		return
	}

	// Cache the user data
	mailboxesJSON := "{}"
	if mailboxes != nil {
		kind10002JSON, _ := json.Marshal(mailboxes)
		mailboxesJSON = string(kind10002JSON)
	}
	kind0JSON, _ := json.Marshal(userMetadata)
	cache.SetUserData(publicKey, string(kind0JSON), mailboxesJSON)
	log.Util().Debug("User data cached successfully", "pubkey", publicKey)

	// Create new session with persistent relay connections
	session, err := SessionMgr.CreateSession(w, publicKey)
	if err != nil {
		log.Util().Error("Failed to create session", "pubkey", publicKey, "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Establish persistent connections to user's relays
	if len(allRelays) > 0 {
		SessionMgr.UpdateUserRelays(SessionMgr.GetSessionToken(r), allRelays)
	}

	log.Util().Info("User logged in successfully with relay connections", 
		"pubkey", session.PublicKey,
		"relay_count", len(allRelays))
}

// SetAppRelays initializes the application relays for initial discovery
func SetAppRelays(relays []string) {
	appRelays = relays
	log.Util().Debug("App relays initialized for discovery", "relay_count", len(relays))
}

// fetchUserMailboxesWithPool fetches mailboxes using a specific relay pool
func fetchUserMailboxesWithPool(publicKey string, relays []string, pool *core.SessionRelayPool) (*core.Mailboxes, error) {
	for _, relayURL := range relays {
		mailboxes, err := fetchMailboxesFromRelayWithPool(publicKey, relayURL, pool)
		if err != nil {
			log.Util().Warn("Failed to fetch mailboxes from relay", "relay", relayURL, "error", err)
			continue
		}
		
		if mailboxes != nil && (len(mailboxes.Read) > 0 || len(mailboxes.Write) > 0 || len(mailboxes.Both) > 0) {
			return mailboxes, nil
		}
	}
	return nil, nil
}

// fetchUserMetadataWithPool fetches metadata using a specific relay pool
func fetchUserMetadataWithPool(publicKey string, relays []string, pool *core.SessionRelayPool) (*nostr.Event, error) {
	for _, relayURL := range relays {
		metadata, err := fetchMetadataFromRelayWithPool(publicKey, relayURL, pool)
		if err != nil {
			log.Util().Warn("Failed to fetch metadata from relay", "relay", relayURL, "error", err)
			continue
		}
		
		if metadata != nil {
			return metadata, nil
		}
	}
	return nil, nil
}

// Helper functions that use a specific pool instead of the global one
func fetchMailboxesFromRelayWithPool(publicKey string, relayURL string, pool *core.SessionRelayPool) (*core.Mailboxes, error) {
	subID := fmt.Sprintf("mailboxes-%s-%d", publicKey[:8], time.Now().UnixNano())
	
	filter := nostr.Filter{
		Authors: []string{publicKey},
		Kinds:   []int{10002},
		Limit:   &[]int{1}[0],
	}

	eventChan, err := pool.Subscribe(relayURL, subID, filter)
	if err != nil {
		return nil, err
	}

	timeout := time.After(5 * time.Second)
	var mailboxes *core.Mailboxes

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				pool.Unsubscribe(relayURL, subID)
				return mailboxes, nil
			}
			
			if mailboxes == nil {
				mailboxes = &core.Mailboxes{}
			}
			
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
						mailboxes.Both = append(mailboxes.Both, relayURL)
					}
				}
			}

		case <-timeout:
			pool.Unsubscribe(relayURL, subID)
			return mailboxes, nil
		}
	}
}

func fetchMetadataFromRelayWithPool(publicKey string, relayURL string, pool *core.SessionRelayPool) (*nostr.Event, error) {
	subID := fmt.Sprintf("metadata-%s-%d", publicKey[:8], time.Now().UnixNano())
	
	filter := nostr.Filter{
		Authors: []string{publicKey},
		Kinds:   []int{0},
		Limit:   &[]int{1}[0],
	}

	eventChan, err := pool.Subscribe(relayURL, subID, filter)
	if err != nil {
		return nil, err
	}

	timeout := time.After(5 * time.Second)
	var latestMetadata *nostr.Event

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				pool.Unsubscribe(relayURL, subID)
				return latestMetadata, nil
			}
			
			if latestMetadata == nil || event.CreatedAt > latestMetadata.CreatedAt {
				latestMetadata = &event
			}

		case <-timeout:
			pool.Unsubscribe(relayURL, subID)
			return latestMetadata, nil
		}
	}
}