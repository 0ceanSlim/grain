package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/server/utils/log"
)

// NpubToPubkeyRequest represents the request structure for npub to pubkey conversion
type NpubToPubkeyRequest struct {
	Npub string `json:"npub"`
}

// NpubToPubkeyResponse represents the response structure for npub to pubkey conversion
type NpubToPubkeyResponse struct {
	Success bool   `json:"success"`
	Npub    string `json:"npub"`
	Pubkey  string `json:"pubkey,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ConvertNpubHandler converts npub to hex pubkey format
func ConvertNpubHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req NpubToPubkeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.ClientAPI().Error("Failed to parse npub convert request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	npub := strings.TrimSpace(req.Npub)
	if npub == "" {
		response := NpubToPubkeyResponse{
			Success: false,
			Error:   "Npub parameter is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.ClientAPI().Debug("Converting npub to pubkey", "npub", npub)

	// Convert npub to hex pubkey
	pubkey, err := tools.DecodeNpub(npub)

	// Prepare response
	response := NpubToPubkeyResponse{
		Success: err == nil,
		Npub:    npub,
	}

	if err != nil {
		log.ClientAPI().Error("Npub to pubkey conversion failed",
			"npub", npub,
			"error", err)
		response.Error = err.Error()
		w.WriteHeader(http.StatusBadRequest)
	} else {
		response.Pubkey = pubkey
		log.ClientAPI().Info("Npub to pubkey conversion successful",
			"npub", npub,
			"pubkey", pubkey)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
