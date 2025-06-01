package utils

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/0ceanslim/grain/server/utils/log"
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
    log.Util().Info("Loading relay metadata", "file", filename)
    
    data, err := os.ReadFile(filename)
    if err != nil {
        log.Util().Error("Failed to read relay metadata file", 
            "file", filename, 
            "error", err)
        return err
    }

    err = json.Unmarshal(data, &relayMetadata)
    if err != nil {
        log.Util().Error("Failed to parse relay metadata JSON", 
            "file", filename, 
            "error", err)
        return err
    }

    log.Util().Info("Relay metadata loaded successfully", 
        "relay_name", relayMetadata.Name, 
        "version", relayMetadata.Version,
        "nips_count", len(relayMetadata.SupportedNIPs))
        
    // Log supported NIPs for debugging
    if len(relayMetadata.SupportedNIPs) > 0 {
        log.Util().Debug("Supported NIPs", "nips", relayMetadata.SupportedNIPs)
    }
    
    return nil
}

func RelayInfoHandler(w http.ResponseWriter, r *http.Request) {
    clientIP := GetClientIP(r)
    
    if r.Header.Get("Accept") != "application/nostr+json" {
        log.Util().Warn("Invalid Accept header for relay info request", 
            "client_ip", clientIP,
            "accept", r.Header.Get("Accept"),
            "path", r.URL.Path)
        http.Error(w, "Unsupported Media Type", http.StatusUnsupportedMediaType)
        return
    }

    log.Util().Debug("Serving relay info", 
        "client_ip", clientIP,
        "user_agent", r.UserAgent())

    w.Header().Set("Content-Type", "application/nostr+json")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
    w.Header().Set("Access-Control-Allow-Methods", "GET")

    err := json.NewEncoder(w).Encode(relayMetadata)
    if err != nil {
        log.Util().Error("Failed to encode relay metadata", 
            "client_ip", clientIP,
            "error", err)
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }
    
    log.Util().Info("Relay info served successfully", 
        "client_ip", clientIP,
        "relay_name", relayMetadata.Name,
        "version", relayMetadata.Version)
}