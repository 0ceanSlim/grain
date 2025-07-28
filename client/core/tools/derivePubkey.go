package tools

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// DerivePublicKey derives a public key from a private key
func DerivePublicKey(privateKeyHex string) (string, error) {
	if len(privateKeyHex) != 64 {
		return "", fmt.Errorf("private key must be 64 hex characters")
	}

	privateKeyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return "", fmt.Errorf("invalid hex private key: %w", err)
	}

	_, publicKey := btcec.PrivKeyFromBytes(privateKeyBytes)
	publicKeyBytes := schnorr.SerializePubKey(publicKey)
	publicKeyHex := hex.EncodeToString(publicKeyBytes)

	return publicKeyHex, nil
}
