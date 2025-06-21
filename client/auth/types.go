// client/auth/types.go
package auth

import (
	"encoding/json"
	"time"

	"github.com/0ceanslim/grain/client/core"
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

// UserCapabilities defines what the user can do in the current session
type UserCapabilities struct {
	CanWrite       bool          `json:"can_write"`
	CanEdit        bool          `json:"can_edit"`
	CanPublish     bool          `json:"can_publish"`
	SigningMethod  SigningMethod `json:"signing_method"`
	ShowEditUI     bool          `json:"show_edit_ui"`
}

// SessionMetadata holds cached user data for the session
type SessionMetadata struct {
	Profile   string `json:"profile"`   // JSON serialized kind 0 event
	Mailboxes string `json:"mailboxes"` // JSON serialized kind 10002 relay list
}

// EnhancedUserSession represents a comprehensive user session
type EnhancedUserSession struct {
	// Core session data
	PublicKey  string    `json:"public_key"`
	LastActive time.Time `json:"last_active"`
	
	// Interaction mode and capabilities
	Mode         SessionInteractionMode `json:"mode"`
	Capabilities UserCapabilities       `json:"capabilities"`
	
	// Cached user data
	Metadata SessionMetadata `json:"metadata"`
	
	// Connection info
	ConnectedRelays []string `json:"connected_relays"`
	
	// Session security
	EncryptedPrivateKey string `json:"encrypted_private_key,omitempty"` // Only if using EncryptedKey method
}

// IsReadOnly returns true if the session is in read-only mode
func (s *EnhancedUserSession) IsReadOnly() bool {
	return s.Mode == ReadOnlyMode
}

// CanCreateEvents returns true if the user can create new events
func (s *EnhancedUserSession) CanCreateEvents() bool {
	return s.Mode == WriteMode && s.Capabilities.CanWrite
}

// GetUserRelays returns the user's relay list as a string slice
func (s *EnhancedUserSession) GetUserRelays() []string {
	if s.Metadata.Mailboxes == "" {
		return s.ConnectedRelays
	}
	
	// Parse mailboxes and return combined read/write relays
	var mailboxes core.Mailboxes
	if err := json.Unmarshal([]byte(s.Metadata.Mailboxes), &mailboxes); err != nil {
		return s.ConnectedRelays
	}
	
	return mailboxes.ToStringSlice()
}

// SessionInitRequest represents data needed to initialize a session
type SessionInitRequest struct {
	PublicKey     string            `json:"public_key"`
	RequestedMode SessionInteractionMode `json:"requested_mode"`
	SigningMethod SigningMethod     `json:"signing_method,omitempty"`
	PrivateKey    string            `json:"private_key,omitempty"` // Only for encrypted key method
}

// SessionResponse represents the response after successful login
type SessionResponse struct {
	Success      bool                `json:"success"`
	Message      string              `json:"message"`
	Session      *EnhancedUserSession `json:"session,omitempty"`
	RedirectURL  string              `json:"redirect_url,omitempty"`
}