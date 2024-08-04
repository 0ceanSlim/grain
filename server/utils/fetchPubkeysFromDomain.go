package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type NostrJSON struct {
	Names map[string]string `json:"names"`
}

func FetchPubkeysFromDomains(domains []string) ([]string, error) {
	var pubkeys []string
	for _, domain := range domains {
		url := fmt.Sprintf("https://%s/.well-known/nostr.json", domain)
		resp, err := http.Get(url)
		if err != nil {
			fmt.Println("Error fetching nostr.json from domain:", domain, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Println("Invalid response from domain:", domain, resp.Status)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading response body from domain:", domain, err)
			continue
		}

		var nostrData NostrJSON
		err = json.Unmarshal(body, &nostrData)
		if err != nil {
			fmt.Println("Error unmarshaling JSON from domain:", domain, err)
			continue
		}

		for _, pubkey := range nostrData.Names {
			pubkeys = append(pubkeys, pubkey)
		}
	}
	return pubkeys, nil
}
