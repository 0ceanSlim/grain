package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	relay "grain/server/types"

	//"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// EscapeSpecialChars escapes special characters in the content according to NIP-01
func EscapeSpecialChars(content string) string {
	content = strings.ReplaceAll(content, "\\", "\\\\")
	content = strings.ReplaceAll(content, "\"", "\\\"")
	content = strings.ReplaceAll(content, "\n", "\\n")
	content = strings.ReplaceAll(content, "\r", "\\r")
	content = strings.ReplaceAll(content, "\t", "\\t")
	content = strings.ReplaceAll(content, "\b", "\\b")
	content = strings.ReplaceAll(content, "\f", "\\f")
	return content
}

// SerializeEvent manually constructs the JSON string for event serialization according to NIP-01
func SerializeEvent(evt relay.Event) string {
	// Escape special characters in the content
	escapedContent := EscapeSpecialChars(evt.Content)

	// Manually construct the event data as a JSON array string
	return fmt.Sprintf(
		`[0,"%s",%d,%d,%s,"%s"]`,
		evt.PubKey,
		evt.CreatedAt,
		evt.Kind,
		serializeTags(evt.Tags),
		escapedContent, // Special characters are escaped
	)
}

// Helper function to serialize the tags array
func serializeTags(tags [][]string) string {
	var tagStrings []string
	for _, tag := range tags {
		tagStrings = append(tagStrings, fmt.Sprintf(`["%s"]`, strings.Join(tag, `","`)))
	}
	return "[" + strings.Join(tagStrings, ",") + "]"
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
