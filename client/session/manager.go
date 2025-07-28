package session

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
)

// Global session manager instance
var SessionMgr *SessionManager

// SessionManager handles comprehensive user authentication and session tracking
type SessionManager struct {
	sessions     map[string]*UserSession
	sessionMutex sync.RWMutex
	cookieName   string
	cookieMaxAge int
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:     make(map[string]*UserSession),
		cookieName:   "grain-session",
		cookieMaxAge: 86400 * 7, // 7 days
	}
}

// GetSessionToken extracts the session token from a request
func (sm *SessionManager) GetSessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sm.cookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// GetUserSession retrieves a user session by token and updates last active time
func (sm *SessionManager) GetUserSession(token string) *UserSession {
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

// CreateSession creates a new lightweight user session (no user data - that goes in cache)
func (sm *SessionManager) CreateSession(w http.ResponseWriter, req SessionInitRequest) (*UserSession, error) {
	token := GenerateRandomToken(32)

	session := &UserSession{
		PublicKey:     req.PublicKey,
		LastActive:    time.Now(),
		Mode:          req.RequestedMode,
		SigningMethod: req.SigningMethod,
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

	log.ClientSession().Info("Created user session",
		"pubkey", req.PublicKey,
		"mode", req.RequestedMode,
		"signing_method", req.SigningMethod,
		"token", token[:8])

	return session, nil
}

// ClearSession removes a user session and clears the cookie
func (sm *SessionManager) ClearSession(w http.ResponseWriter, r *http.Request) {
	token := sm.GetSessionToken(r)
	if token != "" {
		sm.sessionMutex.Lock()
		if session, exists := sm.sessions[token]; exists {
			log.ClientSession().Info("Clearing session",
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
func (sm *SessionManager) GetCurrentUser(r *http.Request) *UserSession {
	token := sm.GetSessionToken(r)
	if token == "" {
		return nil
	}
	return sm.GetUserSession(token)
}

// CleanupSessions removes expired sessions
func (sm *SessionManager) CleanupSessions(maxAge time.Duration) {
	sm.sessionMutex.Lock()
	defer sm.sessionMutex.Unlock()

	now := time.Now()
	cleanedCount := 0

	for token, session := range sm.sessions {
		if now.Sub(session.LastActive) > maxAge {
			delete(sm.sessions, token)
			cleanedCount++
			log.ClientSession().Debug("Cleaned up expired session",
				"pubkey", session.PublicKey,
				"mode", session.Mode)
		}
	}

	if cleanedCount > 0 {
		log.ClientSession().Info("Session cleanup completed", "cleaned_sessions", cleanedCount)
	}
}

// GetSessionStats returns statistics about active sessions
func (sm *SessionManager) GetSessionStats() map[string]interface{} {
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
		signingMethods[session.SigningMethod]++
	}

	return map[string]interface{}{
		"total_sessions":  len(sm.sessions),
		"read_only":       readOnly,
		"write_mode":      writeMode,
		"signing_methods": signingMethods,
	}
}

// Error represents session-related errors
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

// IsSessionManagerInitialized checks if the session manager is properly initialized
func IsSessionManagerInitialized() bool {
	return SessionMgr != nil
}
