package session

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/0ceanslim/grain/client/connection"
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

	// onUserGone overrides what happens when a user's last session ends
	// (default connection.ReleaseUser). A test seam.
	onUserGone func(pubkey string)
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

// GetUserSession retrieves a user session by token and updates last active
// time. It returns a snapshot: the stored session is only ever touched under
// sessionMutex, so concurrent requests on one session don't race.
func (sm *SessionManager) GetUserSession(token string) *UserSession {
	sm.sessionMutex.Lock()
	defer sm.sessionMutex.Unlock()

	session, exists := sm.sessions[token]
	if !exists {
		return nil
	}

	session.LastActive = time.Now()
	snapshot := *session
	return &snapshot
}

// CreateSession creates a new lightweight user session (no user data - that
// goes in cache) and sets its cookie. connectedRelays is recorded on the
// session as-is. Like GetUserSession, it returns a snapshot.
func (sm *SessionManager) CreateSession(w http.ResponseWriter, r *http.Request, req SessionInitRequest, connectedRelays []string) (*UserSession, error) {
	token := GenerateRandomToken(32)

	session := &UserSession{
		PublicKey:       req.PublicKey,
		LastActive:      time.Now(),
		Mode:            req.RequestedMode,
		SigningMethod:   req.SigningMethod,
		ConnectedRelays: connectedRelays,
	}
	snapshot := *session

	sm.sessionMutex.Lock()
	sm.sessions[token] = session
	sm.sessionMutex.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   sm.cookieMaxAge,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteStrictMode,
	})

	log.ClientSession().Info("Created user session",
		"pubkey", req.PublicKey,
		"mode", req.RequestedMode,
		"signing_method", req.SigningMethod,
		"token", token[:8])

	return &snapshot, nil
}

// isHTTPS reports whether the browser reached us over HTTPS, either directly
// or through a TLS-terminating reverse proxy. The session cookie is marked
// Secure exactly then, so it is never sent in the clear but plain-http local
// development still works. Trusting X-Forwarded-Proto is safe here: a client
// that spoofs it only stops its own cookie from being sent over http.
func isHTTPS(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https")
}

// ClearSession removes a user session and clears the cookie
func (sm *SessionManager) ClearSession(w http.ResponseWriter, r *http.Request) {
	token := sm.GetSessionToken(r)
	if token != "" {
		var ended []string
		sm.sessionMutex.Lock()
		if session, exists := sm.sessions[token]; exists {
			log.ClientSession().Info("Clearing session",
				"pubkey", session.PublicKey,
				"mode", session.Mode)
			delete(sm.sessions, token)
			if !sm.hasSessionLocked(session.PublicKey) {
				ended = append(ended, session.PublicKey)
			}
		}
		sm.sessionMutex.Unlock()
		sm.usersEnded(ended)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sm.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isHTTPS(r),
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

	now := time.Now()
	cleanedCount := 0
	expired := make(map[string]bool)

	for token, session := range sm.sessions {
		if now.Sub(session.LastActive) > maxAge {
			delete(sm.sessions, token)
			cleanedCount++
			expired[session.PublicKey] = true
			log.ClientSession().Debug("Cleaned up expired session",
				"pubkey", session.PublicKey,
				"mode", session.Mode)
		}
	}
	var ended []string
	for pubkey := range expired {
		if !sm.hasSessionLocked(pubkey) {
			ended = append(ended, pubkey)
		}
	}
	sm.sessionMutex.Unlock()
	sm.usersEnded(ended)

	if cleanedCount > 0 {
		log.ClientSession().Info("Session cleanup completed", "cleaned_sessions", cleanedCount)
	}
}

// hasSessionLocked reports whether pubkey still has a session. Caller holds
// sessionMutex.
func (sm *SessionManager) hasSessionLocked(pubkey string) bool {
	for _, s := range sm.sessions {
		if s.PublicKey == pubkey {
			return true
		}
	}
	return false
}

// usersEnded releases the per-user client state of users whose last session
// just ended (their held relays, their NIP-42 authenticated connections).
// Called without sessionMutex held.
func (sm *SessionManager) usersEnded(pubkeys []string) {
	release := sm.onUserGone
	if release == nil {
		release = connection.ReleaseUser
	}
	for _, pk := range pubkeys {
		release(pk)
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
