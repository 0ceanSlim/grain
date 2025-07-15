package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/server/utils/log"
)

// GenerateKeypairResponse represents the response structure for key generation
type GenerateKeypairResponse struct {
	Success bool           `json:"success"`
	KeyPair *tools.KeyPair `json:"keypair,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// GenerateKeypairHandler generates a new random Nostr key pair
func GenerateKeypairHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.ClientAPI().Debug("Key pair generation requested")

	// Generate new key pair
	keyPair, err := tools.GenerateKeyPair()

	// Prepare response
	response := GenerateKeypairResponse{
		Success: err == nil,
	}

	if err != nil {
		log.ClientAPI().Error("Key pair generation failed", "error", err)
		response.Error = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		response.KeyPair = keyPair
		log.ClientAPI().Info("Key pair generation successful",
			"pubkey", keyPair.PublicKey,
			"npub", keyPair.Npub)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.ClientAPI().Error("Failed to encode key generation response", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
