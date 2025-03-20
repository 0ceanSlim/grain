package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"

	relay "grain/server/types"

	//"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// SerializeEvent manually constructs the JSON string for event serialization according to NIP-01
func SerializeEvent(evt relay.Event) string {
	eventData := []interface{}{
		0,
		evt.PubKey,
		evt.CreatedAt,
		evt.Kind,
		evt.Tags,
		evt.Content,
	}
	jsonBytes, err := json.Marshal(eventData)
	if err != nil {
		log.Printf("Error serializing event: %v", err)
		return ""
	}
	return string(jsonBytes)
}

// CheckSignature verifies the event's signature and ID
func CheckSignature(evt relay.Event) bool {
	// Serialize event correctly
	serializedEvent := SerializeEvent(evt)
	if serializedEvent == "" {
		log.Printf("Failed to serialize event")
		return false
	}

	// Compute SHA-256 hash
	hash := sha256.Sum256([]byte(serializedEvent))
	eventID := hex.EncodeToString(hash[:])

	// Validate event ID
	if eventID != evt.ID {
		log.Printf("Invalid ID: expected %s, got %s", eventID, evt.ID)
		return false
	}

	// Decode signature
	sigBytes, err := hex.DecodeString(evt.Sig)
	if err != nil || len(sigBytes) != 64 {
		log.Printf("Invalid signature: %v", err)
		return false
	}

	// Parse signature
	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		log.Printf("Error parsing signature: %v", err)
		return false
	}

	// Decode public key
	pubKeyBytes, err := hex.DecodeString(evt.PubKey)
	if err != nil || len(pubKeyBytes) != 32 {
		log.Printf("Invalid public key length: %d", len(pubKeyBytes))
		return false
	}

	// Parse X-only pubkey
	pubKey, err := schnorr.ParsePubKey(pubKeyBytes)
	if err != nil {
		log.Printf("Error parsing public key: %v", err)
		return false
	}

	// Verify signature
	if !sig.Verify(hash[:], pubKey) {
		log.Printf("Signature verification failed for event ID: %s", evt.ID)
		return false
	}

	return true
}
