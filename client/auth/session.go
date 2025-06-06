package auth

import (
	"net/http"
	"sync"
	"time"

	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/core/helpers"
	"github.com/0ceanslim/grain/server/utils/log"
)

// SessionManager handles user authentication and session tracking
type SessionManager struct {
	sessions     map[string]*UserSession
	sessionMutex sync.RWMutex
	cookieName   string
	cookieMaxAge int
}

// UserSession represents an authenticated user session with relay connections
type UserSession struct {
	PublicKey   string
	LastActive  time.Time
	RelayPool   *core.SessionRelayPool
	UserRelays  []string // User's preferred relays from their mailboxes
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

// CreateSession creates a new user session with relay connections
func (sm *SessionManager) CreateSession(w http.ResponseWriter, publicKey string) (*UserSession, error) {
	token := helpers.GenerateRandomToken(32)

	// Create relay pool for this session
	relayPool := core.NewSessionRelayPool(token)

	session := &UserSession{
		PublicKey:  publicKey,
		LastActive: time.Now(),
		RelayPool:  relayPool,
		UserRelays: make([]string, 0),
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

// UpdateUserRelays updates the user's relay list and establishes connections
func (sm *SessionManager) UpdateUserRelays(token string, relays []string) {
	sm.sessionMutex.RLock()
	session, exists := sm.sessions[token]
	sm.sessionMutex.RUnlock()

	if !exists {
		return
	}

	// Disconnect from old relays that are not in the new list
	oldRelays := make(map[string]bool)
	for _, relay := range session.UserRelays {
		oldRelays[relay] = true
	}

	newRelays := make(map[string]bool)
	for _, relay := range relays {
		newRelays[relay] = true
	}

	// Disconnect from relays no longer needed
	for relay := range oldRelays {
		if !newRelays[relay] {
			session.RelayPool.Disconnect(relay)
		}
	}

	// Update session relay list
	session.UserRelays = relays

	// Connect to new relays
	session.RelayPool.ConnectToAll(relays)

	log.Util().Info("Updated user relays", 
		"pubkey", session.PublicKey,
		"relay_count", len(relays))
}

// ClearSession removes a user session and closes relay connections
func (sm *SessionManager) ClearSession(w http.ResponseWriter, r *http.Request) {
	token := sm.GetSessionToken(r)
	if token != "" {
		sm.sessionMutex.Lock()
		if session, exists := sm.sessions[token]; exists {
			// Disconnect from all relays
			session.RelayPool.DisconnectAll()
			log.Util().Info("Closed relay connections for session", 
				"pubkey", session.PublicKey)
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

// CleanupSessions removes expired sessions and closes their relay connections
func (sm *SessionManager) CleanupSessions(maxAge time.Duration) {
	sm.sessionMutex.Lock()
	defer sm.sessionMutex.Unlock()

	now := time.Now()
	for token, session := range sm.sessions {
		if now.Sub(session.LastActive) > maxAge {
			// Disconnect from all relays
			session.RelayPool.DisconnectAll()
			delete(sm.sessions, token)
			log.Util().Info("Cleaned up expired session", 
				"pubkey", session.PublicKey)
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