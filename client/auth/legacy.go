// client/auth/legacy.go
package auth

import (
	"net/http"

	"github.com/0ceanslim/grain/server/utils/log"
)

// LegacyLoginHandler handles form-based login for backward compatibility
// This handler does all the same data fetching and caching as the enhanced system
func LegacyLoginHandler(w http.ResponseWriter, r *http.Request) {
	log.Util().Debug("Legacy login handler called")

	if EnhancedSessionMgr == nil {
		log.Util().Error("EnhancedSessionMgr not initialized")
		http.Error(w, "Session manager not available", http.StatusInternalServerError)
		return
	}

	if coreClient == nil {
		log.Util().Error("Core client not initialized")
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}

	// Check if user is already logged in
	if session := EnhancedSessionMgr.GetCurrentUser(r); session != nil {
		log.Util().Info("User already logged in", "pubkey", session.PublicKey)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Already logged in"))
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

	// Convert form data to session init request
	// Default to read-only mode for legacy requests unless specified
	requestedMode := ReadOnlyMode
	signingMethod := NoSigning
	
	// Check if user specified they want write mode
	if r.FormValue("writeMode") == "true" {
		requestedMode = WriteMode
		// Default to browser extension for write mode
		signingMethod = BrowserExtension
		
		// Allow override of signing method
		if method := r.FormValue("signingMethod"); method != "" {
			signingMethod = SigningMethod(method)
		}
	}

	sessionReq := SessionInitRequest{
		PublicKey:     publicKey,
		RequestedMode: requestedMode,
		SigningMethod: signingMethod,
		PrivateKey:    r.FormValue("privateKey"), // For encrypted key method
	}

	log.Util().Info("Processing legacy login", 
		"pubkey", publicKey,
		"mode", requestedMode,
		"signing_method", signingMethod)

	// Validate the request
	if err := ValidateSessionRequest(sessionReq); err != nil {
		log.Util().Error("Invalid session request", "error", err)
		http.Error(w, "Invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Create session using the SAME enhanced session creation logic
	// This will fetch user metadata, mailboxes, cache everything, and create the session
	session, err := CreateUserSession(w, sessionReq)
	if err != nil {
		log.Util().Error("Failed to create session", "error", err)
		http.Error(w, "Login failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Util().Info("Legacy login successful", 
		"pubkey", publicKey,
		"mode", session.Mode,
		"metadata_cached", session.Metadata.Profile != "",
		"mailboxes_cached", session.Metadata.Mailboxes != "")

	// Return success response for HTMX
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Login successful"))
}

// LegacyLogoutHandler handles logout for backward compatibility
func LegacyLogoutHandler(w http.ResponseWriter, r *http.Request) {
	log.Util().Debug("Legacy logout handler called")

	if EnhancedSessionMgr == nil {
		log.Util().Error("EnhancedSessionMgr not initialized")
		http.Error(w, "Session manager not available", http.StatusInternalServerError)
		return
	}

	// Get current user session
	user := EnhancedSessionMgr.GetCurrentUser(r)
	if user != nil {
		log.Util().Info("User logging out", 
			"pubkey", user.PublicKey,
			"mode", user.Mode)
	}

	// Clear the session
	EnhancedSessionMgr.ClearSession(w, r)

	log.Util().Info("Legacy logout successful")

	// Return success response for HTMX
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Logout successful"))
}