package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/server/utils/log"
)

// KeyDeriveHandler derives public key from private key
func KeyDeriveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract key from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/keys/derive/")
	key := strings.TrimSpace(path)

	if key == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Private key parameter is required"})
		return
	}

	log.ClientAPI().Debug("Deriving public key from private key", "key", key)

	var privateKeyHex string
	var err error

	// Detect input format and convert to hex if needed
	if strings.HasPrefix(key, "nsec") {
		// Convert nsec to hex
		privateKeyHex, err = tools.DecodeNsec(key)
		if err != nil {
			log.ClientAPI().Error("Failed to decode nsec", "input_key", key, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	} else {
		// Assume hex format
		privateKeyHex = key
	}

	// Derive public key from private key
	publicKeyHex, err := tools.DerivePublicKey(privateKeyHex)
	if err != nil {
		log.ClientAPI().Error("Failed to derive public key", "private_key", privateKeyHex, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Convert public key to npub
	npub, err := tools.EncodePubkey(publicKeyHex)
	if err != nil {
		log.ClientAPI().Error("Failed to encode public key to npub", "public_key", publicKeyHex, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Prepare response
	result := map[string]string{
		"public_key": publicKeyHex,
		"npub":       npub,
	}

	log.ClientAPI().Info("Successfully derived public key",
		"input_key", key,
		"public_key", publicKeyHex,
		"npub", npub)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
