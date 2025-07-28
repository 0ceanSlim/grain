package tools

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcutil/bech32"
)

// EncodePrivateKey encodes a hex private key into a Bech32 nsec
func EncodePrivateKey(hexPrivateKey string) (string, error) {
	decoded, err := hex.DecodeString(hexPrivateKey)
	if err != nil {
		return "", fmt.Errorf("invalid hex private key: %w", err)
	}

	if len(decoded) != 32 {
		return "", fmt.Errorf("private key must be 32 bytes")
	}

	encoded, err := bech32.ConvertBits(decoded, 8, 5, true)
	if err != nil {
		return "", fmt.Errorf("failed to convert bits: %w", err)
	}

	nsec, err := bech32.Encode("nsec", encoded)
	if err != nil {
		return "", fmt.Errorf("failed to encode nsec: %w", err)
	}

	return nsec, nil
}
