package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ValidateNpubRequest represents the request structure for npub validation
type ValidateNpubRequest struct {
	Npub string `json:"npub"`
}

// ValidateNpubResponse represents the response structure for npub validation
type ValidateNpubResponse struct {
	Success bool   `json:"success"`
	Npub    string `json:"npub"`
	Valid   bool   `json:"valid"`
	Pubkey  string `json:"pubkey,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ValidateNpubHandler validates npub format and provides pubkey conversion
func ValidateNpubHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ValidateNpubRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Util().Error("Failed to parse npub validate request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	npub := strings.TrimSpace(req.Npub)
	if npub == "" {
		response := ValidateNpubResponse{
			Success: false,
			Error:   "Npub parameter is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Util().Debug("Validating npub", "npub", npub)

	// Try to convert npub to pubkey to validate
	pubkey, err := utils.DecodeNpub(npub)

	// Prepare response
	response := ValidateNpubResponse{
		Success: true,
		Npub:    npub,
		Valid:   err == nil,
	}

	if err != nil {
		log.Util().Debug("Npub validation failed", 
			"npub", npub, 
			"error", err)
		response.Error = err.Error()
	} else {
		response.Pubkey = pubkey
		log.Util().Debug("Npub validation successful", 
			"npub", npub, 
			"pubkey", pubkey)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}