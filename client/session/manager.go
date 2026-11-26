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

	// maxSessions overrides MaxSessions. A test seam.
	maxSessions int
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
		Verified:        req.Verified,
		ConnectedRelays: connectedRelays,
		viewID:          GenerateRandomToken(8),
	}
	snapshot := *session

	sm.sessionMutex.Lock()
	var ended []string
	if len(sm.sessions) >= sm.limit() {
		ended = sm.evictLocked()
	}
	sm.sessions[token] = session
	sm.sessionMutex.Unlock()
	sm.usersEnded(ended)

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
			if !sm.hasSessionLocked(session.UserKey()) {
				ended = append(ended, session.UserKey())
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
			expired[session.UserKey()] = true
			log.ClientSession().Debug("Cleaned up expired session",
				"pubkey", session.PublicKey,
				"mode", session.Mode)
		}
	}
	var ended []string
	for key := range expired {
		if !sm.hasSessionLocked(key) {
			ended = append(ended, key)
		}
	}
	sm.sessionMutex.Unlock()
	sm.usersEnded(ended)

	if cleanedCount > 0 {
		log.ClientSession().Info("Session cleanup completed", "cleaned_sessions", cleanedCount)
	}
}

// MaxSessions caps the session table. Logging in costs nothing but a key
// pair, so without a cap repeated logins grow it without bound. At the cap
// the least recently active session goes, an unverified one first.
const MaxSessions = 10000

func (sm *SessionManager) limit() int {
	if sm.maxSessions > 0 {
		return sm.maxSessions
	}
	return MaxSessions
}

// evictLocked drops the least recently active session, preferring an
// unverified one, and returns the client state it leaves unused. Caller holds
// sessionMutex.
func (sm *SessionManager) evictLocked() []string {
	var victim string
	var oldest *UserSession
	for token, s := range sm.sessions {
		if oldest == nil || (!s.Verified && oldest.Verified) ||
			(s.Verified == oldest.Verified && s.LastActive.Before(oldest.LastActive)) {
			victim, oldest = token, s
		}
	}
	if oldest == nil {
		return nil
	}
	delete(sm.sessions, victim)
	log.ClientSession().Info("Session table full, evicted the least recently active session",
		"pubkey", oldest.PublicKey, "verified", oldest.Verified)
	if sm.hasSessionLocked(oldest.UserKey()) {
		return nil
	}
	return []string{oldest.UserKey()}
}

// hasSessionLocked reports whether a session still uses the client state
// named key (UserKey). Caller holds sessionMutex.
func (sm *SessionManager) hasSessionLocked(key string) bool {
	for _, s := range sm.sessions {
		if s.UserKey() == key {
			return true
		}
	}
	return false
}

// usersEnded releases the per-user client state (by UserKey) whose last
// session just ended: its held relays, its NIP-42 authenticated connections.
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

// HasUserKey reports whether any session uses the client state named key.
func (sm *SessionManager) HasUserKey(key string) bool {
	sm.sessionMutex.RLock()
	defer sm.sessionMutex.RUnlock()
	return sm.hasSessionLocked(key)
}
