package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ValidatePubkeyRequest represents the request structure for pubkey validation
type ValidatePubkeyRequest struct {
	Pubkey string `json:"pubkey"`
}

// ValidatePubkeyResponse represents the response structure for pubkey validation
type ValidatePubkeyResponse struct {
	Success bool   `json:"success"`
	Pubkey  string `json:"pubkey"`
	Valid   bool   `json:"valid"`
	Npub    string `json:"npub,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ValidatePubkeyHandler validates hex pubkey format and provides npub conversion
func ValidatePubkeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get pubkey from query parameter
	pubkey := strings.TrimSpace(r.URL.Query().Get("pubkey"))
	if pubkey == "" {
		response := ValidatePubkeyResponse{
			Success: false,
			Error:   "Pubkey parameter is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.ClientAPI().Debug("Validating pubkey", "pubkey", pubkey)

	// Try to convert pubkey to npub to validate
	npub, err := tools.EncodePubkey(pubkey)

	// Prepare response
	response := ValidatePubkeyResponse{
		Success: true,
		Pubkey:  pubkey,
		Valid:   err == nil,
	}

	if err != nil {
		log.ClientAPI().Debug("Pubkey validation failed",
			"pubkey", pubkey,
			"error", err)
		response.Error = err.Error()
	} else {
		response.Npub = npub
		log.ClientAPI().Debug("Pubkey validation successful",
			"pubkey", pubkey,
			"npub", npub)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
