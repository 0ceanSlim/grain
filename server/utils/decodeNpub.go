package utils

import (
	"encoding/hex"
	"errors"
	"strings"

	"github.com/btcsuite/btcutil/bech32"
)

// DecodeNpub decodes a Bech32 encoded npub to its corresponding pubkey
func DecodeNpub(npub string) (string, error) {
	hrp, data, err := bech32.Decode(npub)
	if err != nil {
		return "", err
	}
	if hrp != "npub" {
		return "", errors.New("invalid hrp")
	}

	decodedData, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		return "", err
	}

	return strings.ToLower(hex.EncodeToString(decodedData)), nil
}
