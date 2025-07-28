package tools

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/0ceanslim/grain/server/utils/log"
)

// KeyPair represents a Nostr key pair
type KeyPair struct {
	PrivateKey string `json:"private_key"` // hex format
	PublicKey  string `json:"public_key"`  // hex format
	Nsec       string `json:"nsec"`        // bech32 format
	Npub       string `json:"npub"`        // bech32 format
}

// GenerateKeyPair generates a new random Nostr key pair
func GenerateKeyPair() (*KeyPair, error) {
	log.ClientTools().Debug("Generating new Nostr key pair")

	// Generate 32 random bytes for private key
	privateKeyBytes := make([]byte, 32)
	if _, err := rand.Read(privateKeyBytes); err != nil {
		log.ClientTools().Error("Failed to generate random private key", "error", err)
		return nil, fmt.Errorf("failed to generate random private key: %w", err)
	}

	// Convert private key to hex
	privateKeyHex := hex.EncodeToString(privateKeyBytes)

	// Derive public key from private key
	publicKeyHex, err := DerivePublicKey(privateKeyHex)
	if err != nil {
		log.ClientTools().Error("Failed to derive public key", "error", err)
		return nil, fmt.Errorf("failed to derive public key: %w", err)
	}

	// Encode private key to nsec format
	nsec, err := EncodePrivateKey(privateKeyHex)
	if err != nil {
		log.ClientTools().Error("Failed to encode private key to nsec", "error", err)
		return nil, fmt.Errorf("failed to encode private key to nsec: %w", err)
	}

	// Encode public key to npub format
	npub, err := EncodePubkey(publicKeyHex)
	if err != nil {
		log.ClientTools().Error("Failed to encode public key to npub", "error", err)
		return nil, fmt.Errorf("failed to encode public key to npub: %w", err)
	}

	keyPair := &KeyPair{
		PrivateKey: privateKeyHex,
		PublicKey:  publicKeyHex,
		Nsec:       nsec,
		Npub:       npub,
	}

	log.ClientTools().Info("Successfully generated Nostr key pair",
		"pubkey", publicKeyHex,
		"npub", npub)

	return keyPair, nil
}
