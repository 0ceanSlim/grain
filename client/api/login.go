package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/session"
	relay "github.com/0ceanslim/grain/server/api"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// LoginHandler handles user login requests via API
// Initializes user by fetching mailboxes, setting app relays, getting metadata from outboxes,
// caching the data, and creating session with appropriate signing capabilities
//
// @Summary      Log in
// @Description  Creates a session for the given pubkey. Fetches mailboxes and metadata from outbox relays as a side effect. `requested_mode=write` needs a signing method and a NIP-98 `Authorization` header (kind 27235 over this URL, `POST`, and the body's sha256) signed by `public_key`; each signed login is accepted once. Without it the session is read-only and unverified: it can view, but never acts as the relay owner or changes the pubkey's relays.
// @Security     NostrAuth
// @Tags         client-auth
// @Accept       json
// @Produce      json
// @Param        body  body      session.SessionInitRequest  true  "Login parameters"
// @Success      200   {object}  session.Response
// @Failure      400   {object}  session.Response  "Invalid request"
// @Failure      401   {string}  string            "Write login missing, invalid, reused, or not signed by public_key"
// @Failure      405   {string}  string            "Method not allowed"
// @Failure      500   {object}  session.Response  "Failed to create session"
// @Router       /api/v1/auth/login [post]
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.ClientAPI().Debug("API login handler called")

	// A write-mode login proves control of its pubkey with a NIP-98 event
	// (kind 27235 over this URL, POST, and the body's hash). Checked before
	// the body is decoded: VerifyAPIAuth reads and restores it.
	var signer string
	if r.Header.Get("Authorization") != "" {
		pk, err := relay.VerifyAPIAuth(r)
		if err != nil {
			log.ClientAPI().Info("Login signature rejected", "client_ip", utils.GetClientIP(r), "error", err)
			http.Error(w, "Invalid login signature: "+err.Error(), http.StatusUnauthorized)
			return
		}
		signer = strings.ToLower(pk)
	}

	// Check if user is already logged in. A signed login for another
	// identity (or upgrading a read-only view) replaces the old session.
	if userSession := session.SessionMgr.GetCurrentUser(r); userSession != nil {
		if signer == "" || (userSession.Verified && userSession.PublicKey == signer) {
			log.ClientAPI().Info("User already logged in", "pubkey", userSession.PublicKey)

			response := session.Response{
				Success: true,
				Message: "Already logged in",
				Session: userSession,
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}
		session.SessionMgr.ClearSession(w, r)
	}

	// Parse JSON request body
	var loginReq session.SessionInitRequest
	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		log.ClientAPI().Error("Failed to parse login request", "error", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if loginReq.PublicKey == "" {
		log.ClientAPI().Warn("Missing publicKey in login request")
		http.Error(w, "Missing publicKey", http.StatusBadRequest)
		return
	}

	loginReq.PublicKey = strings.ToLower(strings.TrimSpace(loginReq.PublicKey))

	// Set defaults if not specified
	if loginReq.RequestedMode == "" {
		loginReq.RequestedMode = session.ReadOnlyMode
		loginReq.SigningMethod = session.NoSigning
	}

	// Validate signing method matches requested mode
	if loginReq.RequestedMode == session.WriteMode && loginReq.SigningMethod == session.NoSigning {
		log.ClientAPI().Warn("Write mode requires signing method", "pubkey", loginReq.PublicKey)
		http.Error(w, "Write mode requires a signing method", http.StatusBadRequest)
		return
	}

	if loginReq.RequestedMode == session.ReadOnlyMode && loginReq.SigningMethod != session.NoSigning {
		log.ClientAPI().Debug("Overriding signing method for read-only mode", "pubkey", loginReq.PublicKey)
		loginReq.SigningMethod = session.NoSigning
	}

	// Only a signed login may write or share the pubkey's client state. An
	// unsigned one is a read-only view of a public profile.
	if loginReq.RequestedMode == session.WriteMode {
		if signer == "" {
			w.Header().Set("WWW-Authenticate", "Nostr")
			http.Error(w, "Write mode requires a NIP-98 signed login", http.StatusUnauthorized)
			return
		}
		if signer != loginReq.PublicKey {
			http.Error(w, "Login signature is not from public_key", http.StatusUnauthorized)
			return
		}
		if !loginOnce.first(r) {
			http.Error(w, "Login signature already used", http.StatusUnauthorized)
			return
		}
		loginReq.Verified = true
	}

	log.ClientAPI().Info("Processing user login",
		"pubkey", loginReq.PublicKey,
		"mode", loginReq.RequestedMode,
		"signing_method", loginReq.SigningMethod)

	// Validate the session request
	if err := session.ValidateSessionRequest(loginReq); err != nil {
		log.ClientAPI().Error("Invalid session request", "error", err)

		response := session.Response{
			Success: false,
			Message: "Invalid request: " + err.Error(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create the session and remember how they logged in. Session creation
	// returns immediately and kicks off user-data discovery in the background
	// (deduped) — login never blocks on outbox relays.
	userSession, err := session.CreateUserSession(w, r, loginReq)
	if err != nil {
		log.ClientAPI().Error("Failed to create session", "error", err)

		response := session.Response{
			Success: false, // Fixed: this should be false on error
			Message: "Failed to create session: " + err.Error(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Check cache for logging (since session no longer has metadata)
	cachedData, hasCachedData := cache.GetUserData(loginReq.PublicKey)

	// Successful login response
	response := session.Response{
		Success: true,
		Message: "Login successful",
		Session: userSession,
	}

	log.ClientAPI().Info("User login successful",
		"pubkey", loginReq.PublicKey,
		"mode", userSession.Mode,
		"signing_method", userSession.SigningMethod,
		"can_create_events", userSession.CanCreateEvents(),
		"cached_profile", hasCachedData && cachedData.Metadata != "",
		"cached_mailboxes", hasCachedData && cachedData.Mailboxes != "" && cachedData.Mailboxes != "{}")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// loginOnce remembers the signed login events seen inside NIP-98's time
// window, so a captured Authorization header can't mint a second session.
var loginOnce = &seenLogins{seen: make(map[string]time.Time)}

type seenLogins struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

// first reports whether r's NIP-98 event is new, and records it.
func (s *seenLogins) first(r *http.Request) bool {
	evt, err := relay.ExtractNIP98Event(r)
	if err != nil {
		return false
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for sig, at := range s.seen {
		if now.Sub(at) > 2*time.Minute { // twice the NIP-98 window
			delete(s.seen, sig)
		}
	}
	if _, dup := s.seen[evt.Sig]; dup {
		return false
	}
	s.seen[evt.Sig] = now
	return true
}
