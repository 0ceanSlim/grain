package utils

import (
	"encoding/hex"

	"github.com/btcsuite/btcutil/bech32"
)

// EncodeNpub encodes a hex public key into a Bech32 npub
func EncodeNpub(hexPubKey string) (string, error) {
	decoded, err := hex.DecodeString(hexPubKey)
	if err != nil {
		return "", err
	}

	encoded, err := bech32.ConvertBits(decoded, 8, 5, true)
	if err != nil {
		return "", err
	}

	return bech32.Encode("npub", encoded)
}