package session

import (
	"time"
)

// SessionInteractionMode defines how the user interacts with the app
type SessionInteractionMode string

const (
	// ReadOnlyMode allows viewing content only
	ReadOnlyMode SessionInteractionMode = "read_only"
	// WriteMode allows creating/publishing events
	WriteMode SessionInteractionMode = "write"
)

// SigningMethod defines how events are signed
type SigningMethod string

const (
	// BrowserExtension uses browser-based Nostr extensions
	BrowserExtension SigningMethod = "browser_extension"
	// AmberSigning uses Amber on Android for signing
	AmberSigning SigningMethod = "amber"
	// BunkerSigning uses remote signing bunkers
	BunkerSigning SigningMethod = "bunker"
	// EncryptedKey uses an encrypted private key stored in session
	EncryptedKey SigningMethod = "encrypted_key"
	// NoSigning for read-only mode
	NoSigning SigningMethod = "none"
)

// UserSession represents a lightweight user session (no user data - that's in cache)
type UserSession struct {
	// Core session data
	PublicKey  string    `json:"public_key"`
	LastActive time.Time `json:"last_active"`

	// Interaction mode and signing
	Mode          SessionInteractionMode `json:"mode"`
	SigningMethod SigningMethod          `json:"signing_method"`

	// Connection info (app-level relays, not user-specific)
	ConnectedRelays []string `json:"connected_relays"`

	// Session security
	EncryptedPrivateKey string `json:"encrypted_private_key,omitempty"` // Only if using EncryptedKey method
}

// IsReadOnly returns true if the session is in read-only mode
func (s *UserSession) IsReadOnly() bool {
	return s.Mode == ReadOnlyMode
}

// CanCreateEvents returns true if the user can create new events
func (s *UserSession) CanCreateEvents() bool {
	return s.Mode == WriteMode
}

// CanSign returns true if the user can sign events
func (s *UserSession) CanSign() bool {
	return s.Mode == WriteMode && s.SigningMethod != NoSigning
}

// SessionInitRequest represents data needed to initialize a session
type SessionInitRequest struct {
	PublicKey     string                 `json:"public_key"`
	RequestedMode SessionInteractionMode `json:"requested_mode"`
	SigningMethod SigningMethod          `json:"signing_method,omitempty"`
	PrivateKey    string                 `json:"private_key,omitempty"` // Only for encrypted key method
}

// Response represents the response after successful login
type Response struct {
	Success bool         `json:"success"`
	Message string       `json:"message"`
	Session *UserSession `json:"session,omitempty"`
}
