package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
)

// EnhancedSessionManager handles comprehensive user authentication and session tracking
type EnhancedSessionManager struct {
	sessions     map[string]*EnhancedUserSession
	sessionMutex sync.RWMutex
	cookieName   string
	cookieMaxAge int
}

// NewEnhancedSessionManager creates a new enhanced session manager
func NewEnhancedSessionManager() *EnhancedSessionManager {
	return &EnhancedSessionManager{
		sessions:     make(map[string]*EnhancedUserSession),
		cookieName:   "grain-session",
		cookieMaxAge: 86400 * 7, // 7 days
	}
}

// GetSessionToken extracts the session token from a request
func (sm *EnhancedSessionManager) GetSessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sm.cookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// GetUserSession retrieves a user session by token and updates last active time
func (sm *EnhancedSessionManager) GetUserSession(token string) *EnhancedUserSession {
	sm.sessionMutex.RLock()
	defer sm.sessionMutex.RUnlock()

	session, exists := sm.sessions[token]
	if !exists {
		return nil
	}

	// Update last active time
	session.LastActive = time.Now()
	return session
}

// CreateSession creates a new comprehensive user session
func (sm *EnhancedSessionManager) CreateSession(w http.ResponseWriter, req SessionInitRequest, metadata SessionMetadata) (*EnhancedUserSession, error) {
	token := GenerateRandomToken(32)

	// Determine capabilities based on signing method
	capabilities := UserCapabilities{
		SigningMethod: req.SigningMethod,
		CanWrite:      req.RequestedMode == WriteMode,
		CanEdit:       req.RequestedMode == WriteMode,
		CanPublish:    req.RequestedMode == WriteMode && req.SigningMethod != NoSigning,
		ShowEditUI:    req.RequestedMode == WriteMode,
	}

	session := &EnhancedUserSession{
		PublicKey:    req.PublicKey,
		LastActive:   time.Now(),
		Mode:         req.RequestedMode,
		Capabilities: capabilities,
		Metadata:     metadata,
		ConnectedRelays: []string{}, // Will be populated during login
	}

	// Store encrypted private key if provided
	if req.PrivateKey != "" && req.SigningMethod == EncryptedKey {
		// In a real implementation, this should be properly encrypted
		session.EncryptedPrivateKey = req.PrivateKey
	}

	sm.sessionMutex.Lock()
	sm.sessions[token] = session
	sm.sessionMutex.Unlock()

	// Set secure cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   sm.cookieMaxAge,
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
	})

	log.Util().Info("Created enhanced user session", 
		"pubkey", req.PublicKey,
		"mode", req.RequestedMode,
		"signing_method", req.SigningMethod,
		"token", token[:8])

	return session, nil
}

// UpdateSessionCapabilities updates the capabilities of an existing session
func (sm *EnhancedSessionManager) UpdateSessionCapabilities(token string, capabilities UserCapabilities) error {
	sm.sessionMutex.Lock()
	defer sm.sessionMutex.Unlock()

	session, exists := sm.sessions[token]
	if !exists {
		return &SessionError{Message: "session not found"}
	}

	session.Capabilities = capabilities
	session.LastActive = time.Now()

	log.Util().Debug("Updated session capabilities", 
		"pubkey", session.PublicKey,
		"can_write", capabilities.CanWrite,
		"signing_method", capabilities.SigningMethod)

	return nil
}

// UpdateSessionMetadata updates cached metadata for a session
func (sm *EnhancedSessionManager) UpdateSessionMetadata(token string, metadata SessionMetadata) error {
	sm.sessionMutex.Lock()
	defer sm.sessionMutex.Unlock()

	session, exists := sm.sessions[token]
	if !exists {
		return &SessionError{Message: "session not found"}
	}

	session.Metadata = metadata
	session.LastActive = time.Now()

	log.Util().Debug("Updated session metadata", "pubkey", session.PublicKey)
	return nil
}

// ClearSession removes a user session and clears the cookie
func (sm *EnhancedSessionManager) ClearSession(w http.ResponseWriter, r *http.Request) {
	token := sm.GetSessionToken(r)
	if token != "" {
		sm.sessionMutex.Lock()
		if session, exists := sm.sessions[token]; exists {
			log.Util().Info("Clearing session", 
				"pubkey", session.PublicKey,
				"mode", session.Mode)
		}
		delete(sm.sessions, token)
		sm.sessionMutex.Unlock()
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
	})
}

// GetCurrentUser retrieves the current user session from the request
func (sm *EnhancedSessionManager) GetCurrentUser(r *http.Request) *EnhancedUserSession {
	token := sm.GetSessionToken(r)
	if token == "" {
		return nil
	}
	return sm.GetUserSession(token)
}

// CleanupSessions removes expired sessions
func (sm *EnhancedSessionManager) CleanupSessions(maxAge time.Duration) {
	sm.sessionMutex.Lock()
	defer sm.sessionMutex.Unlock()

	now := time.Now()
	cleanedCount := 0
	
	for token, session := range sm.sessions {
		if now.Sub(session.LastActive) > maxAge {
			delete(sm.sessions, token)
			cleanedCount++
			log.Util().Debug("Cleaned up expired session", 
				"pubkey", session.PublicKey,
				"mode", session.Mode)
		}
	}

	if cleanedCount > 0 {
		log.Util().Info("Session cleanup completed", "cleaned_sessions", cleanedCount)
	}
}

// GetSessionStats returns statistics about active sessions
func (sm *EnhancedSessionManager) GetSessionStats() map[string]interface{} {
	sm.sessionMutex.RLock()
	defer sm.sessionMutex.RUnlock()

	readOnly := 0
	writeMode := 0
	signingMethods := make(map[SigningMethod]int)

	for _, session := range sm.sessions {
		if session.Mode == ReadOnlyMode {
			readOnly++
		} else {
			writeMode++
		}
		signingMethods[session.Capabilities.SigningMethod]++
	}

	return map[string]interface{}{
		"total_sessions":   len(sm.sessions),
		"read_only":        readOnly,
		"write_mode":       writeMode,
		"signing_methods":  signingMethods,
	}
}

// SessionError represents session-related errors
type SessionError struct {
	Message string
}

func (e *SessionError) Error() string {
	return "session error: " + e.Message
}

// GenerateRandomToken creates a cryptographically secure random token
func GenerateRandomToken(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		// Fallback to time-based token
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}