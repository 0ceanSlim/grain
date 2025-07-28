package tools

import (
	"encoding/hex"
	"errors"
	"strings"

	"github.com/0ceanslim/grain/server/utils/log"
	"github.com/btcsuite/btcutil/bech32"
)

// DecodeNsec decodes a Bech32 encoded nsec to its corresponding hex private key
func DecodeNsec(nsec string) (string, error) {
	log.ClientTools().Debug("Decoding nsec", "nsec", nsec)

	hrp, data, err := bech32.Decode(nsec)
	if err != nil {
		log.ClientTools().Error("Failed to decode bech32 nsec", "nsec", nsec, "error", err)
		return "", err
	}

	if hrp != "nsec" {
		log.ClientTools().Error("Invalid hrp in bech32 decode", "nsec", nsec, "hrp", hrp, "expected", "nsec")
		return "", errors.New("invalid hrp")
	}

	decodedData, err := bech32.ConvertBits(data, 5, 8, false)
	if err != nil {
		log.ClientTools().Error("Failed to convert bits", "nsec", nsec, "error", err)
		return "", err
	}

	if len(decodedData) != 32 {
		log.ClientTools().Error("Invalid decoded nsec length", "nsec", nsec, "length", len(decodedData), "expected", 32)
		return "", errors.New("invalid private key length")
	}

	privateKey := strings.ToLower(hex.EncodeToString(decodedData))
	log.ClientTools().Debug("Successfully decoded nsec",
		"nsec", nsec,
		"private_key_length", len(privateKey))

	return privateKey, nil
}
