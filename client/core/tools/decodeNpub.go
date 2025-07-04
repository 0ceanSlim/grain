package tools

import (
	"encoding/hex"
	"errors"
	"strings"

	"github.com/0ceanslim/grain/server/utils/log"
	"github.com/btcsuite/btcutil/bech32"
)

// DecodeNpub decodes a Bech32 encoded npub to its corresponding pubkey
func DecodeNpub(npub string) (string, error) {
	log.ClientTools().Debug("Decoding npub", "npub", npub)

	hrp, data, err := bech32.Decode(npub)
	if err != nil {
		log.ClientTools().Error("Failed to decode bech32 npub", "npub", npub, "error", err)
		return "", err
	}

	if hrp != "npub" {
		log.ClientTools().Error("Invalid hrp in bech32 decode", "npub", npub, "hrp", hrp, "expected", "npub")
		return "", errors.New("invalid hrp")
	}

	decodedData, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		log.ClientTools().Error("Failed to convert bits", "npub", npub, "error", err)
		return "", err
	}

	pubkey := strings.ToLower(hex.EncodeToString(decodedData))
	log.ClientTools().Debug("Successfully decoded npub",
		"npub", npub,
		"pubkey", pubkey,
		"pubkey_length", len(pubkey))

	return pubkey, nil
}
