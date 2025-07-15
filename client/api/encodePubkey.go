package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/server/utils/log"
)

// PubkeyToNpubRequest represents the request structure for pubkey to npub conversion
type PubkeyToNpubRequest struct {
	Pubkey string `json:"pubkey"`
}

// PubkeyToNpubResponse represents the response structure for pubkey to npub conversion
type PubkeyToNpubResponse struct {
	Success bool   `json:"success"`
	Pubkey  string `json:"pubkey"`
	Npub    string `json:"npub,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ConvertPubkeyHandler converts hex pubkey to npub format
func ConvertPubkeyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get pubkey from query parameter
	pubkey := strings.TrimSpace(r.URL.Query().Get("pubkey"))
	if pubkey == "" {
		response := PubkeyToNpubResponse{
			Success: false,
			Error:   "Pubkey parameter is required",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	log.ClientAPI().Debug("Converting pubkey to npub", "pubkey", pubkey)

	// Convert hex pubkey to npub
	npub, err := tools.EncodePubkey(pubkey)

	// Prepare response
	response := PubkeyToNpubResponse{
		Success: err == nil,
		Pubkey:  pubkey,
	}

	if err != nil {
		log.ClientAPI().Error("Pubkey to npub conversion failed",
			"pubkey", pubkey,
			"error", err)
		response.Error = err.Error()
		w.WriteHeader(http.StatusBadRequest)
	} else {
		response.Npub = npub
		log.ClientAPI().Info("Pubkey to npub conversion successful",
			"pubkey", pubkey,
			"npub", npub)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
