package api

import (
	"encoding/json"
	"net/http"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/server/utils/log"
)

// KeyGenerationResponse represents the response structure for key generation
type KeyGenerationResponse struct {
	PrivateKey string `json:"private_key,omitempty"` // hex format
	PublicKey  string `json:"public_key,omitempty"`  // hex format
	Nsec       string `json:"nsec,omitempty"`        // bech32 format
	Npub       string `json:"npub,omitempty"`        // bech32 format
	Error      string `json:"error,omitempty"`
}

// KeyGenerationHandler generates a new random Nostr key pair
//
// @Summary      Generate keypair
// @Description  Generates a fresh secp256k1 keypair and returns both the hex and bech32 (nsec/npub) forms. Pure utility — the relay does not store the private key.
// @Tags         client-keys
// @Produce      json
// @Success      200  {object}  KeyGenerationResponse
// @Failure      405  {string}  string                "Method not allowed"
// @Failure      500  {object}  KeyGenerationResponse "Key pair generation failed"
// @Router       /api/v1/keys/generate [get]
func KeyGenerationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.ClientAPI().Debug("Generating new key pair")

	// Generate new key pair
	keyPair, err := tools.GenerateKeyPair()

	// Prepare response
	var response KeyGenerationResponse

	if err != nil {
		log.ClientAPI().Error("Key pair generation failed", "error", err)
		response.Error = err.Error()
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		response.PrivateKey = keyPair.PrivateKey
		response.PublicKey = keyPair.PublicKey
		response.Nsec = keyPair.Nsec
		response.Npub = keyPair.Npub
		log.ClientAPI().Info("Key pair generation successful",
			"pubkey", keyPair.PublicKey,
			"npub", keyPair.Npub)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
