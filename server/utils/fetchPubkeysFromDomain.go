package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const cacheFile = "app/static/domain_pubkey_cache.json"

type NostrJSON struct {
	Names map[string]string `json:"names"`
}

type CachedDomains struct {
	Timestamp int64               `json:"timestamp"`  // Store Unix timestamp
	Domains   map[string][]string `json:"domains"`   // Cached pubkeys per domain
}

// FetchPubkeysFromDomains fetches nostr.json pubkeys from multiple domains with caching.
func FetchPubkeysFromDomains(domains []string) ([]string, error) {
	var pubkeys []string

	// Load cache
	cache := loadDomainCache()

	// Loop through each domain
	for _, domain := range domains {
		url := fmt.Sprintf("https://%s/.well-known/nostr.json", domain)
		client := http.Client{Timeout: 5 * time.Second}

		resp, err := client.Get(url)
		if err != nil {
			fmt.Println("Error fetching nostr.json from domain:", domain, err)
			// Use cached pubkeys if available
			if cachedKeys, exists := cache.Domains[domain]; exists {
				fmt.Println("Using cached pubkeys for domain:", domain)
				pubkeys = append(pubkeys, cachedKeys...)
			}
			continue
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			fmt.Println("Invalid response from domain:", domain, resp.Status)
			continue
		}

		// Read body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading response body from domain:", domain, err)
			continue
		}

		// Parse JSON
		var nostrData NostrJSON
		err = json.Unmarshal(body, &nostrData)
		if err != nil {
			fmt.Println("Error unmarshaling JSON from domain:", domain, err)
			continue
		}

		// Extract pubkeys
		var domainPubkeys []string
		for _, pubkey := range nostrData.Names {
			domainPubkeys = append(domainPubkeys, pubkey)
			pubkeys = append(pubkeys, pubkey)
		}

		// Update cache
		cache.Domains[domain] = domainPubkeys
		cache.Timestamp = time.Now().Unix()
	}

	// Save cache
	saveDomainCache(cache)

	return pubkeys, nil
}

// loadDomainCache loads the cached pubkeys from file.
func loadDomainCache() CachedDomains {
	cache := CachedDomains{
		Timestamp: time.Now().Unix(), // Default to current time
		Domains:   make(map[string][]string),
	}

	// Check if file exists
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		return cache // Return empty cache if file doesn't exist
	}

	// Read file
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		fmt.Println("Error reading cache file:", err)
		return cache
	}

	// Parse JSON
	err = json.Unmarshal(data, &cache)
	if err != nil {
		fmt.Println("Error parsing cache file:", err)
		return cache
	}

	return cache
}

// saveDomainCache writes the cached pubkeys to file.
func saveDomainCache(cache CachedDomains) {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling cache:", err)
		return
	}

	err = os.WriteFile(cacheFile, data, 0644)
	if err != nil {
		fmt.Println("Error writing cache file:", err)
	}
}
