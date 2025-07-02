package validation

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// CheckSignature verifies the event's signature and ID
func CheckSignature(evt nostr.Event) bool {
	// Serialize event correctly
	serializedEvent := core.SerializeEvent(evt)
	if serializedEvent == "" {
		log.Validation().Error("Failed to serialize event", 
			"event_id", evt.ID, 
			"pubkey", evt.PubKey)
		return false
	}

	// Compute SHA-256 hash
	hash := sha256.Sum256([]byte(serializedEvent))
	eventID := hex.EncodeToString(hash[:])

	// Validate event ID
	if eventID != evt.ID {
		log.Validation().Error("Invalid event ID", 
			"expected", eventID, 
			"actual", evt.ID, 
			"pubkey", evt.PubKey,
			"kind", evt.Kind)
		return false
	}

	// Decode signature
	sigBytes, err := hex.DecodeString(evt.Sig)
	if err != nil || len(sigBytes) != 64 {
		log.Validation().Error("Invalid signature format", 
			"event_id", evt.ID, 
			"pubkey", evt.PubKey, 
			"sig_length", len(evt.Sig),
			"error", err)
		return false
	}

	// Parse signature
	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		log.Validation().Error("Failed to parse signature", 
			"event_id", evt.ID, 
			"pubkey", evt.PubKey, 
			"error", err)
		return false
	}

	// Decode public key
	pubKeyBytes, err := hex.DecodeString(evt.PubKey)
	if err != nil || len(pubKeyBytes) != 32 {
		log.Validation().Error("Invalid public key", 
			"event_id", evt.ID, 
			"pubkey", evt.PubKey, 
			"pubkey_length", len(pubKeyBytes),
			"error", err)
		return false
	}

	// Parse X-only pubkey
	pubKey, err := schnorr.ParsePubKey(pubKeyBytes)
	if err != nil {
		log.Validation().Error("Failed to parse public key", 
			"event_id", evt.ID, 
			"pubkey", evt.PubKey, 
			"error", err)
		return false
	}

	// Verify signature
	if !sig.Verify(hash[:], pubKey) {
		log.Validation().Error("Signature verification failed", 
			"event_id", evt.ID, 
			"pubkey", evt.PubKey,
			"kind", evt.Kind)
		return false
	}

	// Debug log for successful verification
	// Commented out to avoid excessive logging for normal operations
	// log.Validation().Debug("Signature verified successfully", "event_id", evt.ID, "pubkey", evt.PubKey)
	
	return true
}