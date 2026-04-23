package utils

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateChallenge creates a random hex string for NIP-42 authentication
func GenerateChallenge(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
