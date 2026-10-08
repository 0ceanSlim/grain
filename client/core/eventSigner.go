package core

import (
	"encoding/hex"
	"fmt"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// EventSigner handles event signing with private keys
type EventSigner struct {
	privateKey *btcec.PrivateKey
	publicKey  string
}

// NewEventSigner creates a new event signer from a hex private key
func NewEventSigner(privateKeyHex string) (*EventSigner, error) {
	if len(privateKeyHex) != 64 {
		return nil, fmt.Errorf("private key must be 64 hex characters")
	}

	keyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid hex private key: %w", err)
	}

	privateKey, publicKey := btcec.PrivKeyFromBytes(keyBytes)

	// Get public key in hex format
	pubKeyBytes := schnorr.SerializePubKey(publicKey)
	pubKeyHex := hex.EncodeToString(pubKeyBytes)

	signer := &EventSigner{
		privateKey: privateKey,
		publicKey:  pubKeyHex,
	}

	clog().Debug("Event signer created", "pubkey", pubKeyHex)
	return signer, nil
}

// NewEventSignerFromRandom creates a new event signer with a random private key
func NewEventSignerFromRandom() (*EventSigner, error) {
	privateKey, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Convert to hex
	privateKeyBytes := privateKey.Serialize()
	privateKeyHex := hex.EncodeToString(privateKeyBytes)

	return NewEventSigner(privateKeyHex)
}

// SignEvent signs an event and sets the ID, PubKey, and Sig fields
func (es *EventSigner) SignEvent(event *nostr.Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Set the public key
	event.PubKey = es.publicKey

	// Compute and set the event ID
	eventID, err := ComputeEventID(event)
	if err != nil {
		return fmt.Errorf("failed to compute event ID: %w", err)
	}
	event.ID = eventID

	// Sign the event ID
	signature, err := es.signHash(eventID)
	if err != nil {
		return fmt.Errorf("failed to sign event: %w", err)
	}
	event.Sig = signature

	clog().Debug("Event signed", "event_id", eventID, "pubkey", es.publicKey)
	return nil
}

// signHash signs a hex-encoded hash
func (es *EventSigner) signHash(hashHex string) (string, error) {
	hashBytes, err := hex.DecodeString(hashHex)
	if err != nil {
		return "", fmt.Errorf("invalid hash hex: %w", err)
	}

	signature, err := schnorr.Sign(es.privateKey, hashBytes)
	if err != nil {
		return "", fmt.Errorf("schnorr signature failed: %w", err)
	}

	return hex.EncodeToString(signature.Serialize()), nil
}

// PublicKey returns the public key in hex format. It satisfies the [Signer]
// seam; GetPublicKey is retained as an alias for existing callers.
func (es *EventSigner) PublicKey() string {
	return es.publicKey
}

// GetPublicKey returns the public key in hex format
func (es *EventSigner) GetPublicKey() string {
	return es.publicKey
}

// GetPrivateKeyHex returns the private key in hex format (use carefully!)
func (es *EventSigner) GetPrivateKeyHex() string {
	return hex.EncodeToString(es.privateKey.Serialize())
}

// VerifyEvent checks that event.ID is the NIP-01 hash of the event's contents
// and that event.Sig is a valid BIP-340 signature of that id by event.PubKey.
// It does not log, so it is safe on the inbound path where a remote relay
// controls the input.
func VerifyEvent(event *nostr.Event) error {
	if event == nil {
		return fmt.Errorf("nil event")
	}
	if len(event.ID) != 64 || len(event.PubKey) != 64 || len(event.Sig) != 128 {
		return fmt.Errorf("id, pubkey or sig has the wrong length")
	}

	computedID, err := ComputeEventID(event)
	if err != nil {
		return fmt.Errorf("compute id: %w", err)
	}
	if event.ID != computedID {
		return fmt.Errorf("id does not match contents")
	}

	pubKeyBytes, err := hex.DecodeString(event.PubKey)
	if err != nil {
		return fmt.Errorf("pubkey is not hex")
	}
	publicKey, err := schnorr.ParsePubKey(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("invalid pubkey: %w", err)
	}
	sigBytes, err := hex.DecodeString(event.Sig)
	if err != nil {
		return fmt.Errorf("sig is not hex")
	}
	signature, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		return fmt.Errorf("invalid sig: %w", err)
	}
	hashBytes, err := hex.DecodeString(event.ID)
	if err != nil {
		return fmt.Errorf("id is not hex")
	}
	if !signature.Verify(hashBytes, publicKey) {
		return fmt.Errorf("signature does not verify")
	}
	return nil
}

// VerifyEventSignature verifies an event's id and signature, logging why it
// failed. Prefer VerifyEvent for input from untrusted sources.
func VerifyEventSignature(event *nostr.Event) bool {
	if err := VerifyEvent(event); err != nil {
		id := ""
		if event != nil {
			id = event.ID
		}
		clog().Warn("Event verification failed", "event_id", id, "error", err)
		return false
	}
	clog().Debug("Event signature verified", "event_id", event.ID)
	return true
}

// Browser extension integration functions

// SignEventWithExtension attempts to sign an event using browser extension (NIP-07)
func SignEventWithExtension(event *nostr.Event) error {
	// This is a placeholder for browser extension integration
	// In a real implementation, this would use JavaScript bridge to call window.nostr.signEvent()
	clog().Warn("Browser extension signing not implemented - this is a server-side client")
	return fmt.Errorf("browser extension signing not available in server environment")
}

// GetPublicKeyFromExtension attempts to get public key from browser extension
func GetPublicKeyFromExtension() (string, error) {
	// This is a placeholder for browser extension integration
	// In a real implementation, this would use JavaScript bridge to call window.nostr.getPublicKey()
	clog().Warn("Browser extension key retrieval not implemented - this is a server-side client")
	return "", fmt.Errorf("browser extension not available in server environment")
}

// Utility functions

// GeneratePrivateKey generates a new random private key
func GeneratePrivateKey() (string, error) {
	privateKey, err := btcec.NewPrivateKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate private key: %w", err)
	}

	privateKeyBytes := privateKey.Serialize()
	return hex.EncodeToString(privateKeyBytes), nil
}

// DerivePublicKey derives a public key from a private key hex
func DerivePublicKey(privateKeyHex string) (string, error) {
	signer, err := NewEventSigner(privateKeyHex)
	if err != nil {
		return "", err
	}
	return signer.GetPublicKey(), nil
}
