package auth

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Global session manager instance
var SessionMgr *SessionManager

// Global core client instance
var coreClient *core.Client

// Application relays for initial discovery
var appRelays []string

// InitializeCoreClient sets up the global core client with retry
func InitializeCoreClient(relays []string) error {
	config := core.DefaultConfig()
	config.DefaultRelays = relays
	
	coreClient = core.NewClient(config)
	
	// Connect to default relays with retry
	if err := coreClient.ConnectToRelaysWithRetry(relays, 3); err != nil {
		log.Util().Error("Failed to connect to relays after retries", "error", err)
		return err
	}
	
	log.Util().Info("Core client initialized", "relay_count", len(relays))
	return nil
}

// LoginHandler handles user login and session initialization using core client
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	log.Util().Debug("Login handler called")

	if SessionMgr == nil {
		log.Util().Error("SessionMgr not initialized")
		http.Error(w, "Session manager not available", http.StatusInternalServerError)
		return
	}
	
	if coreClient == nil {
		log.Util().Error("Core client not initialized")
		http.Error(w, "Client not available", http.StatusInternalServerError)
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

	// Fetch fresh data using core client
	if err := fetchAndCacheUserDataWithCoreClient(publicKey); err != nil {
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

// fetchAndCacheUserDataWithCoreClient fetches user data using the core client
func fetchAndCacheUserDataWithCoreClient(publicKey string) error {
	log.Util().Debug("Fetching fresh user data with core client", "pubkey", publicKey)

	// Ensure we have connected relays before proceeding
	if err := ensureRelayConnections(); err != nil {
		return fmt.Errorf("failed to ensure relay connections: %w", err)
	}

	// First, try to get user's mailboxes using connected relays
	var relaysForMetadata []string
	mailboxes, err := coreClient.GetUserRelays(publicKey)
	if err != nil {
		log.Util().Warn("Failed to fetch mailboxes, using app relays", "pubkey", publicKey, "error", err)
		relaysForMetadata = appRelays
	} else if mailboxes != nil {
		// Get user's preferred relays
		userRelays := mailboxes.ToStringSlice()
		log.Util().Debug("User has preferred relays", "pubkey", publicKey, "relay_count", len(userRelays))
		
		// BUT: Use connected app relays for profile fetch to ensure success
		// This is more reliable than trying to connect to user's personal relays
		connectedRelays := coreClient.GetConnectedRelays()
		if len(connectedRelays) > 0 {
			relaysForMetadata = connectedRelays
			log.Util().Debug("Using connected app relays for metadata", "pubkey", publicKey, "relay_count", len(relaysForMetadata))
		} else {
			relaysForMetadata = appRelays
		}
	}

	// Use app relays as fallback
	if len(relaysForMetadata) == 0 {
		relaysForMetadata = appRelays
		log.Util().Info("Using app relays for metadata", "pubkey", publicKey, "relay_count", len(relaysForMetadata))
	}

	// Fetch user metadata (profile) using connected relays
	userMetadata, err := coreClient.GetUserProfile(publicKey, relaysForMetadata)
	if err != nil || userMetadata == nil {
		return fmt.Errorf("failed to fetch user metadata: %w", err)
	}

	// Cache the data
	cacheUserData(publicKey, userMetadata, mailboxes)

	log.Util().Info("User data fetched and cached successfully", "pubkey", publicKey)
	return nil
}

// ensureRelayConnections checks and reconnects to relays if needed
func ensureRelayConnections() error {
	if coreClient == nil {
		return fmt.Errorf("core client not initialized")
	}
	
	// Check current connections
	connectedRelays := coreClient.GetConnectedRelays()
	log.Util().Debug("Current relay connections", "connected_count", len(connectedRelays))
	
	// If we have some connections, we're good
	if len(connectedRelays) > 0 {
		return nil
	}
	
	// No connections, try to reconnect
	log.Util().Warn("No relay connections found, attempting to reconnect")
	
	if err := coreClient.ConnectToRelaysWithRetry(appRelays, 3); err != nil {
		log.Util().Error("Failed to reconnect to relays", "error", err)
		return err
	}
	
	// Verify we now have connections
	connectedRelays = coreClient.GetConnectedRelays()
	if len(connectedRelays) == 0 {
		return fmt.Errorf("still no relay connections after reconnection attempt")
	}
	
	log.Util().Info("Successfully reconnected to relays", "connected_count", len(connectedRelays))
	return nil
}

// GetUserProfile retrieves user profile data using core client with cache fallback
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
	
	// Fetch fresh data using core client
	log.Util().Debug("Cache miss, fetching fresh profile data", "pubkey", publicKey)
	
	mailboxes, err = coreClient.GetUserRelays(publicKey)
	if err != nil {
		log.Util().Warn("Failed to fetch mailboxes", "pubkey", publicKey, "error", err)
	}
	
	relaysForMetadata := appRelays
	if mailboxes != nil {
		relaysForMetadata = mailboxes.ToStringSlice()
	}
	
	metadata, err = coreClient.GetUserProfile(publicKey, relaysForMetadata)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch profile: %w", err)
	}
	
	// Cache the fresh data
	cacheUserData(publicKey, metadata, mailboxes)
	
	return metadata, mailboxes, nil
}

// RebuildCacheForSession rebuilds cache for an existing session using core client
func RebuildCacheForSession(session *UserSession) {
	log.Util().Info("Rebuilding cache for existing session", "pubkey", session.PublicKey)
	
	go func() {
		// Fetch data in background to avoid blocking the request
		if err := fetchAndCacheUserDataWithCoreClient(session.PublicKey); err != nil {
			log.Util().Error("Failed to rebuild cache for session", 
				"pubkey", session.PublicKey, "error", err)
		} else {
			log.Util().Info("Cache rebuilt successfully for session", 
				"pubkey", session.PublicKey)
		}
	}()
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

// GetCoreClientStatus returns status information about the core client
func GetCoreClientStatus() map[string]interface{} {
	if coreClient == nil {
		return map[string]interface{}{
			"initialized": false,
			"error": "core client not initialized",
		}
	}
	
	connectedRelays := coreClient.GetConnectedRelays()
	
	return map[string]interface{}{
		"initialized": true,
		"connected_relays": connectedRelays,
		"connected_count": len(connectedRelays),
		"app_relays": appRelays,
	}
}

// ReinitializeCoreClient reinitializes the core client (for recovery)
func ReinitializeCoreClient() error {
	log.Util().Warn("Reinitializing core client")
	
	// Close existing client if any
	if coreClient != nil {
		coreClient.Close()
	}
	
	// Reinitialize
	return InitializeCoreClient(appRelays)
}
func GetCoreClient() *core.Client {
	return coreClient
}

// CloseCoreClient shuts down the core client
func CloseCoreClient() error {
	if coreClient != nil {
		return coreClient.Close()
	}
	return nil
}