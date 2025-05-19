package validation

import (
	"crypto/sha256"
	"encoding/hex"

	relay "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// CheckSignature verifies the event's signature and ID
func CheckSignature(evt relay.Event) bool {
	// Serialize event correctly
	serializedEvent := utils.SerializeEvent(evt)
	if serializedEvent == "" {
		validationLog().Error("Failed to serialize event", 
			"event_id", evt.ID, 
			"pubkey", evt.PubKey)
		return false
	}

	// Compute SHA-256 hash
	hash := sha256.Sum256([]byte(serializedEvent))
	eventID := hex.EncodeToString(hash[:])

	// Validate event ID
	if eventID != evt.ID {
		validationLog().Error("Invalid event ID", 
			"expected", eventID, 
			"actual", evt.ID, 
			"pubkey", evt.PubKey,
			"kind", evt.Kind)
		return false
	}

	// Decode signature
	sigBytes, err := hex.DecodeString(evt.Sig)
	if err != nil || len(sigBytes) != 64 {
		validationLog().Error("Invalid signature format", 
			"event_id", evt.ID, 
			"pubkey", evt.PubKey, 
			"sig_length", len(evt.Sig),
			"error", err)
		return false
	}

	// Parse signature
	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		validationLog().Error("Failed to parse signature", 
			"event_id", evt.ID, 
			"pubkey", evt.PubKey, 
			"error", err)
		return false
	}

	// Decode public key
	pubKeyBytes, err := hex.DecodeString(evt.PubKey)
	if err != nil || len(pubKeyBytes) != 32 {
		validationLog().Error("Invalid public key", 
			"event_id", evt.ID, 
			"pubkey", evt.PubKey, 
			"pubkey_length", len(pubKeyBytes),
			"error", err)
		return false
	}

	// Parse X-only pubkey
	pubKey, err := schnorr.ParsePubKey(pubKeyBytes)
	if err != nil {
		validationLog().Error("Failed to parse public key", 
			"event_id", evt.ID, 
			"pubkey", evt.PubKey, 
			"error", err)
		return false
	}

	// Verify signature
	if !sig.Verify(hash[:], pubKey) {
		validationLog().Error("Signature verification failed", 
			"event_id", evt.ID, 
			"pubkey", evt.PubKey,
			"kind", evt.Kind)
		return false
	}

	// Debug log for successful verification
	// Commented out to avoid excessive logging for normal operations
	// validationLog().Debug("Signature verified successfully", "event_id", evt.ID, "pubkey", evt.PubKey)
	
	return true
}