package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Nip19DecodeRequest represents the request body for NIP-19 decoding
type Nip19DecodeRequest struct {
	Entity string `json:"entity"`
}

// Nip19DecodeHandler decodes NIP-19 bech32 entities (npub, note, nprofile, nevent, naddr)
// Accepts both GET (URL path) and POST (JSON body) requests to handle long entities
//
// @Summary      Decode NIP-19 entity
// @Description  Decodes any NIP-19 bech32 entity (npub, nsec, note, nprofile, nevent, naddr). POST accepts entities too long for a URL path.
// @Tags         client-keys
// @Accept       json
// @Produce      json
// @Param        entity  path      string              false  "NIP-19 entity (GET path form)"
// @Param        body    body      Nip19DecodeRequest  false  "NIP-19 entity (POST body form)"
// @Success      200     {object}  map[string]interface{}
// @Failure      400     {object}  map[string]string  "Invalid entity"
// @Failure      405     {string}  string             "Method not allowed"
// @Router       /api/v1/keys/decode/nip19/{entity} [get]
func Nip19DecodeHandler(w http.ResponseWriter, r *http.Request) {
	var entity string

	switch r.Method {
	case http.MethodGet:
		// Extract entity from URL path
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/keys/decode/nip19/")
		entity = strings.TrimSpace(path)

	case http.MethodPost:
		// Extract entity from JSON body
		var req Nip19DecodeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.ClientAPI().Error("Failed to parse NIP-19 decode request", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}
		entity = strings.TrimSpace(req.Entity)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if entity == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "NIP-19 entity parameter is required"})
		return
	}

	log.ClientAPI().Debug("Decoding NIP-19 entity",
		"entity", entity,
		"method", r.Method,
		"length", len(entity))

	// Decode the NIP-19 entity
	decoded, err := tools.DecodeNip19Entity(entity)
	if err != nil {
		log.ClientAPI().Error("NIP-19 entity decoding failed",
			"entity", entity,
			"length", len(entity),
			"error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	log.ClientAPI().Info("NIP-19 entity decoding successful",
		"entity", entity,
		"type", decoded.Type,
		"data", decoded.Data,
		"relays_count", len(decoded.Relays))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(decoded)
}
