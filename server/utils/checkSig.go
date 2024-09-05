package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	relay "grain/server/types"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// SerializeEvent manually constructs the JSON string for event serialization
func SerializeEvent(evt relay.Event) string {
	// Manually construct the event data as a JSON array string
	// Avoid escaping special characters like "&"
	return fmt.Sprintf(
		`[0,"%s",%d,%d,%s,"%s"]`,
		evt.PubKey,
		evt.CreatedAt,
		evt.Kind,
		serializeTags(evt.Tags),
		evt.Content, // Special characters like "&" are not escaped here
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
	// Manually serialize the event
	serializedEvent := SerializeEvent(evt)

	// Compute the SHA-256 hash of the serialized event
	hash := sha256.Sum256([]byte(serializedEvent))
	eventID := hex.EncodeToString(hash[:])

	// Log the generated and provided IDs
	log.Printf("Generated event ID: %s, Provided event ID: %s", eventID, evt.ID)

	// Compare the computed event ID with the one provided by the client
	if eventID != evt.ID {
		log.Printf("Invalid ID: expected %s, got %s", eventID, evt.ID)
		return false
	}

	// Decode the signature from hex
	sigBytes, err := hex.DecodeString(evt.Sig)
	if err != nil {
		log.Printf("Error decoding signature: %v", err)
		return false
	}

	// Parse the Schnorr signature
	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		log.Printf("Error parsing signature: %v", err)
		return false
	}

	// Decode the public key from hex
	pubKeyBytes, err := hex.DecodeString(evt.PubKey)
	if err != nil {
		log.Printf("Error decoding public key: %v", err)
		return false
	}

	// Parse the public key based on its length
	var pubKey *btcec.PublicKey
	if len(pubKeyBytes) == 32 {
		// Handle 32-byte compressed public key (x-coordinate only)
		pubKey, err = btcec.ParsePubKey(append([]byte{0x02}, pubKeyBytes...))
	} else if len(pubKeyBytes) == 33 || len(pubKeyBytes) == 65 {
		// Handle standard compressed (33-byte) or uncompressed (65-byte) public key
		pubKey, err = btcec.ParsePubKey(pubKeyBytes)
	} else {
		log.Printf("Malformed public key: invalid length: %d", len(pubKeyBytes))
		return false
	}

	if err != nil {
		log.Printf("Error parsing public key: %v", err)
		return false
	}

	// Verify the signature using the event's hash and public key
	verified := sig.Verify(hash[:], pubKey)
	if !verified {
		log.Printf("Signature verification failed for event ID: %s", evt.ID)
	}

	return verified
}
