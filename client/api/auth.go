// client/api/auth.go
package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/auth"
	"github.com/0ceanslim/grain/server/utils/log"
)

// LoginHandler handles user login requests via API
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Util().Debug("API login handler called")

	// Check if user is already logged in
	if session := auth.EnhancedSessionMgr.GetCurrentUser(r); session != nil {
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

	// Default to read-only mode if not specified
	if loginReq.RequestedMode == "" {
		loginReq.RequestedMode = auth.ReadOnlyMode
		loginReq.SigningMethod = auth.NoSigning
	}

	log.Util().Info("Processing API login", 
		"pubkey", loginReq.PublicKey,
		"mode", loginReq.RequestedMode,
		"signing_method", loginReq.SigningMethod)

	// Delegate to auth package for session creation
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
		RedirectURL: "/profile",
	}

	log.Util().Info("API login successful", 
		"pubkey", loginReq.PublicKey,
		"mode", session.Mode)

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
	user := auth.EnhancedSessionMgr.GetCurrentUser(r)
	if user != nil {
		log.Util().Info("User logging out", 
			"pubkey", user.PublicKey,
			"mode", user.Mode)
	}

	// Clear the session
	auth.EnhancedSessionMgr.ClearSession(w, r)

	response := map[string]interface{}{
		"success": true,
		"message": "Logout successful",
	}

	log.Util().Info("API logout successful")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

