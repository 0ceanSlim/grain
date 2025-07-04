package core

import (
	"encoding/hex"
	"fmt"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
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
	
	log.ClientCore().Debug("Event signer created", "pubkey", pubKeyHex)
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
	
	log.ClientCore().Debug("Event signed", "event_id", eventID, "pubkey", es.publicKey)
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

// GetPublicKey returns the public key in hex format
func (es *EventSigner) GetPublicKey() string {
	return es.publicKey
}

// GetPrivateKeyHex returns the private key in hex format (use carefully!)
func (es *EventSigner) GetPrivateKeyHex() string {
	return hex.EncodeToString(es.privateKey.Serialize())
}

// VerifyEventSignature verifies an event's signature
func VerifyEventSignature(event *nostr.Event) bool {
	if event == nil {
		log.ClientCore().Warn("Cannot verify nil event")
		return false
	}
	
	if event.ID == "" || event.PubKey == "" || event.Sig == "" {
		log.ClientCore().Warn("Event missing required fields for verification", 
			"has_id", event.ID != "",
			"has_pubkey", event.PubKey != "",
			"has_sig", event.Sig != "")
		return false
	}
	
	// Verify the event ID matches the computed ID
	computedID, err := ComputeEventID(event)
	if err != nil {
		log.ClientCore().Error("Failed to compute event ID for verification", "error", err)
		return false
	}
	
	if event.ID != computedID {
		log.ClientCore().Warn("Event ID mismatch", "event_id", event.ID, "computed_id", computedID)
		return false
	}
	
	// Parse public key
	pubKeyBytes, err := hex.DecodeString(event.PubKey)
	if err != nil {
		log.ClientCore().Error("Invalid public key hex", "pubkey", event.PubKey, "error", err)
		return false
	}
	
	publicKey, err := schnorr.ParsePubKey(pubKeyBytes)
	if err != nil {
		log.ClientCore().Error("Failed to parse public key", "error", err)
		return false
	}
	
	// Parse signature
	sigBytes, err := hex.DecodeString(event.Sig)
	if err != nil {
		log.ClientCore().Error("Invalid signature hex", "signature", event.Sig, "error", err)
		return false
	}
	
	signature, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		log.ClientCore().Error("Failed to parse signature", "error", err)
		return false
	}
	
	// Parse event ID as hash
	hashBytes, err := hex.DecodeString(event.ID)
	if err != nil {
		log.ClientCore().Error("Invalid event ID hex", "event_id", event.ID, "error", err)
		return false
	}
	
	// Verify signature
	valid := signature.Verify(hashBytes, publicKey)
	
	if valid {
		log.ClientCore().Debug("Event signature verified", "event_id", event.ID)
	} else {
		log.ClientCore().Warn("Event signature verification failed", "event_id", event.ID)
	}
	
	return valid
}

// Browser extension integration functions

// SignEventWithExtension attempts to sign an event using browser extension (NIP-07)
func SignEventWithExtension(event *nostr.Event) error {
	// This is a placeholder for browser extension integration
	// In a real implementation, this would use JavaScript bridge to call window.nostr.signEvent()
	log.ClientCore().Warn("Browser extension signing not implemented - this is a server-side client")
	return fmt.Errorf("browser extension signing not available in server environment")
}

// GetPublicKeyFromExtension attempts to get public key from browser extension
func GetPublicKeyFromExtension() (string, error) {
	// This is a placeholder for browser extension integration
	// In a real implementation, this would use JavaScript bridge to call window.nostr.getPublicKey()
	log.ClientCore().Warn("Browser extension key retrieval not implemented - this is a server-side client")
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