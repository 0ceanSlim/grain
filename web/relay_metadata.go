package web

import (
	"encoding/json"
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
