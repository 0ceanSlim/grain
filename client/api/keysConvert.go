package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/server/utils/log"
)

// PublicKeyConversionHandler converts between hex and npub formats
func PublicKeyConversionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract key from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/keys/convert/public/")
	key := strings.TrimSpace(path)

	if key == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Key parameter is required"})
		return
	}

	log.ClientAPI().Debug("Converting public key", "key", key)

	var result map[string]string
	var err error

	// Detect input format and convert
	if strings.HasPrefix(key, "npub") {
		// Convert npub to hex
		hexKey, convertErr := tools.DecodeNpub(key)
		if convertErr != nil {
			err = convertErr
		} else {
			result = map[string]string{"public_key": hexKey}
		}
	} else {
		// Assume hex, convert to npub
		npubKey, convertErr := tools.EncodePubkey(key)
		if convertErr != nil {
			err = convertErr
		} else {
			result = map[string]string{"npub": npubKey}
		}
	}

	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		log.ClientAPI().Error("Public key conversion failed", "input_key", key, "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	} else {
		log.ClientAPI().Info("Public key conversion successful", "input_key", key, "result", result)
		json.NewEncoder(w).Encode(result)
	}
}

// PrivateKeyConversionHandler converts between hex and nsec formats
func PrivateKeyConversionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract key from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/keys/convert/private/")
	key := strings.TrimSpace(path)

	if key == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Key parameter is required"})
		return
	}

	log.ClientAPI().Debug("Converting private key", "key", key)

	var result map[string]string
	var err error

	// Detect input format and convert
	if strings.HasPrefix(key, "nsec") {
		// Convert nsec to hex
		hexKey, convertErr := tools.DecodeNsec(key)
		if convertErr != nil {
			err = convertErr
		} else {
			result = map[string]string{"private_key": hexKey}
		}
	} else {
		// Assume hex, convert to nsec
		nsecKey, convertErr := tools.EncodePrivateKey(key)
		if convertErr != nil {
			err = convertErr
		} else {
			result = map[string]string{"nsec": nsecKey}
		}
	}

	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		log.ClientAPI().Error("Private key conversion failed", "input_key", key, "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
	} else {
		log.ClientAPI().Info("Private key conversion successful", "input_key", key, "result", result)
		json.NewEncoder(w).Encode(result)
	}
}
