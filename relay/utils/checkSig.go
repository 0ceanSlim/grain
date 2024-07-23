package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"

	relay "grain/relay/types"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)
func SerializeEvent(evt relay.Event) []byte {
	eventData := []interface{}{
		0,
		evt.PubKey,
		evt.CreatedAt,
		evt.Kind,
		evt.Tags,
		evt.Content,
	}
	serializedEvent, _ := json.Marshal(eventData)
	return serializedEvent
}

func CheckSignature(evt relay.Event) bool {
	serializedEvent := SerializeEvent(evt)
	hash := sha256.Sum256(serializedEvent)
	eventID := hex.EncodeToString(hash[:])
	if eventID != evt.ID {
		log.Printf("Invalid ID: expected %s, got %s\n", eventID, evt.ID)
		return false
	}

	sigBytes, err := hex.DecodeString(evt.Sig)
	if err != nil {
		log.Printf("Error decoding signature: %v\n", err)
		return false
	}

	sig, err := schnorr.ParseSignature(sigBytes)
	if err != nil {
		log.Printf("Error parsing signature: %v\n", err)
		return false
	}

	pubKeyBytes, err := hex.DecodeString(evt.PubKey)
	if err != nil {
		log.Printf("Error decoding public key: %v\n", err)
		return false
	}

	var pubKey *btcec.PublicKey
	if len(pubKeyBytes) == 32 {
		// Handle 32-byte public key (x-coordinate only)
		pubKey, err = btcec.ParsePubKey(append([]byte{0x02}, pubKeyBytes...))
	} else {
		// Handle standard compressed or uncompressed public key
		pubKey, err = btcec.ParsePubKey(pubKeyBytes)
	}
	if err != nil {
		log.Printf("Error parsing public key: %v\n", err)
		return false
	}

	verified := sig.Verify(hash[:], pubKey)
	if !verified {
		log.Printf("Signature verification failed for event ID: %s\n", evt.ID)
	}

	return verified
}
