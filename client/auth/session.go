package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
)

// SessionManager handles user authentication and session tracking
type SessionManager struct {
	sessions     map[string]*UserSession
	sessionMutex sync.RWMutex
	cookieName   string
	cookieMaxAge int
}

// UserSession represents a lightweight authenticated user session
type UserSession struct {
	PublicKey  string
	LastActive time.Time
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

// GetUserSession retrieves a user session by token
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

// CreateSession creates a new lightweight user session
func (sm *SessionManager) CreateSession(w http.ResponseWriter, publicKey string) (*UserSession, error) {
	token := GenerateRandomToken(32)

	session := &UserSession{
		PublicKey:  publicKey,
		LastActive: time.Now(),
	}

	sm.sessionMutex.Lock()
	sm.sessions[token] = session
	sm.sessionMutex.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   sm.cookieMaxAge,
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
	})

	log.Util().Info("Created user session", 
		"pubkey", publicKey,
		"token", token[:8]) // Log only first 8 chars for security

	return session, nil
}

// ClearSession removes a user session
func (sm *SessionManager) ClearSession(w http.ResponseWriter, r *http.Request) {
	token := sm.GetSessionToken(r)
	if token != "" {
		sm.sessionMutex.Lock()
		if session, exists := sm.sessions[token]; exists {
			log.Util().Info("Clearing session", "pubkey", session.PublicKey)
		}
		delete(sm.sessions, token)
		sm.sessionMutex.Unlock()
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
	})
}

// CleanupSessions removes expired sessions
func (sm *SessionManager) CleanupSessions(maxAge time.Duration) {
	sm.sessionMutex.Lock()
	defer sm.sessionMutex.Unlock()

	now := time.Now()
	for token, session := range sm.sessions {
		if now.Sub(session.LastActive) > maxAge {
			delete(sm.sessions, token)
			log.Util().Info("Cleaned up expired session", "pubkey", session.PublicKey)
		}
	}
}

// GetCurrentUser retrieves the current user from the session
func (sm *SessionManager) GetCurrentUser(r *http.Request) *UserSession {
	token := sm.GetSessionToken(r)
	if token == "" {
		return nil
	}
	return sm.GetUserSession(token)
}

// GenerateRandomToken creates a cryptographically secure random token
// of the specified length in bytes (output will be twice this length as hex)
func GenerateRandomToken(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		// In a real application, handle this error better
		// For now, let's log and generate something less secure but still random
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}
