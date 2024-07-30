package web

import (
	"encoding/json"
	"net/http"
	"os"
)

type RelayMetadata struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Pubkey        string `json:"pubkey"`
	Contact       string `json:"contact"`
	SupportedNIPs []int  `json:"supported_nips"`
	Software      string `json:"software"`
	Version       string `json:"version"`
}

var relayMetadata RelayMetadata

func LoadRelayMetadataJSON() error {
	return LoadRelayMetadata("relay_metadata.json")
}

func LoadRelayMetadata(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &relayMetadata)
	if err != nil {
		return err
	}

	return nil
}

func RelayInfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Accept") != "application/nostr+json" {
		http.Error(w, "Unsupported Media Type", http.StatusUnsupportedMediaType)
		return
	}

	w.Header().Set("Content-Type", "application/nostr+json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	json.NewEncoder(w).Encode(relayMetadata)
}
