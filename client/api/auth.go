package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/auth"
	"github.com/0ceanslim/grain/server/utils/log"
)

// LoginHandler handles user login requests via API
// Initializes user by fetching mailboxes, setting app relays, getting metadata from outboxes,
// caching the data, and creating session with appropriate signing capabilities
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Util().Debug("API login handler called")

	// Check if user is already logged in
	if session := auth.SessionMgr.GetCurrentUser(r); session != nil {
		log.Util().Info("User already logged in", "pubkey", session.PublicKey)
		
		response := auth.SessionResponse{
			Success: true,
			Message: "Already logged in",
			Session: session,
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Parse JSON request body
	var loginReq auth.SessionInitRequest
	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		log.Util().Error("Failed to parse login request", "error", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if loginReq.PublicKey == "" {
		log.Util().Warn("Missing publicKey in login request")
		http.Error(w, "Missing publicKey", http.StatusBadRequest)
		return
	}

	// Set defaults if not specified
	if loginReq.RequestedMode == "" {
		loginReq.RequestedMode = auth.ReadOnlyMode
		loginReq.SigningMethod = auth.NoSigning
	}

	// Validate signing method matches requested mode
	if loginReq.RequestedMode == auth.WriteMode && loginReq.SigningMethod == auth.NoSigning {
		log.Util().Warn("Write mode requires signing method", "pubkey", loginReq.PublicKey)
		http.Error(w, "Write mode requires a signing method", http.StatusBadRequest)
		return
	}

	if loginReq.RequestedMode == auth.ReadOnlyMode && loginReq.SigningMethod != auth.NoSigning {
		log.Util().Debug("Overriding signing method for read-only mode", "pubkey", loginReq.PublicKey)
		loginReq.SigningMethod = auth.NoSigning
	}

	log.Util().Info("Processing user login", 
		"pubkey", loginReq.PublicKey,
		"mode", loginReq.RequestedMode,
		"signing_method", loginReq.SigningMethod)

	// Validate the session request
	if err := auth.ValidateSessionRequest(loginReq); err != nil {
		log.Util().Error("Invalid session request", "error", err)
		
		response := auth.SessionResponse{
			Success: false,
			Message: "Invalid request: " + err.Error(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Initialize user data: fetch mailboxes, set app relays, get metadata from outboxes, cache everything
	log.Util().Debug("Fetching and caching user data", "pubkey", loginReq.PublicKey)
	
	if err := auth.FetchAndCacheUserDataWithCoreClient(loginReq.PublicKey); err != nil {
		log.Util().Warn("Failed to fetch user data, proceeding with session creation", 
			"pubkey", loginReq.PublicKey, "error", err)
		// Continue with session creation even if fetch fails - user might be new or relays unavailable
	}

	// Create session with the fetched/cached data and remember how they logged in
	session, err := auth.CreateUserSession(w, loginReq)
	if err != nil {
		log.Util().Error("Failed to create session", "error", err)
		
		response := auth.SessionResponse{
			Success: false,
			Message: "Login failed: " + err.Error(),
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Successful login response
	response := auth.SessionResponse{
		Success:     true,
		Message:     "Login successful",
		Session:     session,
	}

	log.Util().Info("User login successful", 
		"pubkey", loginReq.PublicKey,
		"mode", session.Mode,
		"signing_method", session.Capabilities.SigningMethod,
		"can_sign", session.Capabilities.CanWrite,
		"cached_profile", session.Metadata.Profile != "",
		"cached_mailboxes", session.Metadata.Mailboxes != "")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// LogoutHandler handles user logout requests via API
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Util().Debug("API logout handler called")

	// Get current user session
	user := auth.SessionMgr.GetCurrentUser(r)
	if user != nil {
		log.Util().Info("User logging out", 
			"pubkey", user.PublicKey,
			"mode", user.Mode,
			"signing_method", user.Capabilities.SigningMethod)
	}

	// Clear the session
	auth.SessionMgr.ClearSession(w, r)

	response := map[string]interface{}{
		"success": true,
		"message": "Logout successful",
	}

	log.Util().Info("API logout successful")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}