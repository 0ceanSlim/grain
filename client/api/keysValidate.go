package api

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/server/utils/log"
)

// KeyValidationResponse represents the response structure for key validation
type KeyValidationResponse struct {
	Valid bool   `json:"valid"`
	Type  string `json:"type"`
	Error string `json:"error,omitempty"`
}

// KeyValidationHandler validates any key type (hex, npub, or nsec)
func KeyValidationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract key from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/keys/validate/")
	key := strings.TrimSpace(path)

	if key == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Key parameter is required"})
		return
	}

	log.ClientAPI().Debug("Validating key", "key", key)

	valid, keyType := validateKey(key)

	response := KeyValidationResponse{
		Valid: valid,
		Type:  keyType,
	}

	if !valid {
		response.Error = "Invalid key format"
		log.ClientAPI().Debug("Key validation failed", "key", key, "type", keyType)
	} else {
		log.ClientAPI().Debug("Key validation successful", "key", key, "type", keyType)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// validateKey determines the key type and validates it
func validateKey(key string) (bool, string) {
	key = strings.TrimSpace(key)

	// Check for npub (public key in bech32 format)
	if strings.HasPrefix(key, "npub") {
		_, err := tools.DecodeNpub(key)
		return err == nil, "npub"
	}

	// Check for nsec (private key in bech32 format)
	if strings.HasPrefix(key, "nsec") {
		_, err := tools.DecodeNsec(key)
		return err == nil, "nsec"
	}

	// Assume hex format - determine if it's a public or private key
	if isValidHex(key) {
		if len(key) == 64 {
			// Could be either public or private key in hex
			// Try to encode as public key first
			_, err := tools.EncodePubkey(key)
			if err == nil {
				return true, "hex"
			}

			// Try to encode as private key
			_, err = tools.EncodePrivateKey(key)
			if err == nil {
				return true, "hex"
			}
		}
	}

	// Invalid key
	return false, "unknown"
}

// isValidHex checks if a string is valid hexadecimal
func isValidHex(s string) bool {
	if len(s) == 0 {
		return false
	}

	_, err := hex.DecodeString(s)
	return err == nil
}
