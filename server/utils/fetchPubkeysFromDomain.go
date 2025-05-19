package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const cacheFile = "www/static/domain_pubkey_cache.json"

// Domain types
type NostrJSON struct {
	Names map[string]string `json:"names"`
}

type CachedDomains struct {
	Timestamp int64               `json:"timestamp"`  // Store Unix timestamp
	Domains   map[string][]string `json:"domains"`   // Cached pubkeys per domain
}

// FetchPubkeysFromDomains fetches nostr.json pubkeys from multiple domains with caching.
func FetchPubkeysFromDomains(domains []string) ([]string, error) {
	utilLog.Info("Fetching pubkeys from domains", "domain_count", len(domains))
	var pubkeys []string

	// Load cache
	cache := loadDomainCache()

	// Loop through each domain
	for _, domain := range domains {
		url := fmt.Sprintf("https://%s/.well-known/nostr.json", domain)
		utilLog.Debug("Fetching nostr.json", "domain", domain, "url", url)
		
		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(url)
		
		if err != nil {
			utilLog.Warn("Error fetching nostr.json", 
				"domain", domain, 
				"error", err)
				
			// Use cached pubkeys if available
			if cachedKeys, exists := cache.Domains[domain]; exists {
				utilLog.Info("Using cached pubkeys", 
					"domain", domain, 
					"pubkey_count", len(cachedKeys),
					"cache_age_seconds", time.Now().Unix() - cache.Timestamp)
				pubkeys = append(pubkeys, cachedKeys...)
			}
			continue
		}
		defer resp.Body.Close()

		// Check response status
		if resp.StatusCode != http.StatusOK {
			utilLog.Warn("Invalid HTTP response", 
				"domain", domain, 
				"status", resp.Status, 
				"status_code", resp.StatusCode)
			continue
		}

		// Read body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			utilLog.Error("Error reading response body", 
				"domain", domain, 
				"error", err)
			continue
		}

		// Parse JSON
		var nostrData NostrJSON
		err = json.Unmarshal(body, &nostrData)
		if err != nil {
			utilLog.Error("Error unmarshaling JSON", 
				"domain", domain, 
				"error", err, 
				"body_size", len(body))
			continue
		}

		// Extract pubkeys
		var domainPubkeys []string
		for name, pubkey := range nostrData.Names {
			domainPubkeys = append(domainPubkeys, pubkey)
			pubkeys = append(pubkeys, pubkey)
			utilLog.Debug("Found pubkey in domain", 
				"domain", domain, 
				"name", name, 
				"pubkey", pubkey)
		}

		utilLog.Info("Successfully fetched pubkeys", 
			"domain", domain, 
			"pubkey_count", len(domainPubkeys))

		// Update cache
		cache.Domains[domain] = domainPubkeys
		cache.Timestamp = time.Now().Unix()
	}

	// Save cache
	saveDomainCache(cache)

	utilLog.Info("Completed domain pubkey fetch", 
		"total_domains", len(domains), 
		"total_pubkeys", len(pubkeys))
	return pubkeys, nil
}

// loadDomainCache loads the cached pubkeys from file.
func loadDomainCache() CachedDomains {
	utilLog.Debug("Loading domain cache", "cache_file", cacheFile)
	
	cache := CachedDomains{
		Timestamp: time.Now().Unix(), // Default to current time
		Domains:   make(map[string][]string),
	}

	// Check if file exists
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		utilLog.Info("Cache file does not exist, using empty cache", "cache_file", cacheFile)
		return cache // Return empty cache if file doesn't exist
	}

	// Read file
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		utilLog.Error("Error reading cache file", 
			"file", cacheFile, 
			"error", err)
		return cache
	}

	// Parse JSON
	err = json.Unmarshal(data, &cache)
	if err != nil {
		utilLog.Error("Error parsing cache file", 
			"file", cacheFile, 
			"error", err)
		return cache
	}

	cacheAge := time.Now().Unix() - cache.Timestamp
	utilLog.Debug("Cache loaded successfully", 
		"domains", len(cache.Domains), 
		"age_seconds", cacheAge,
		"age_hours", cacheAge/3600)
	return cache
}

// saveDomainCache writes the cached pubkeys to file.
func saveDomainCache(cache CachedDomains) {
	utilLog.Debug("Saving domain cache", 
		"cache_file", cacheFile, 
		"domains", len(cache.Domains))
		
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		utilLog.Error("Error marshaling cache", "error", err)
		return
	}

	err = os.WriteFile(cacheFile, data, 0644)
	if err != nil {
		utilLog.Error("Error writing cache file", 
			"file", cacheFile, 
			"error", err)
		return
	}
	
	utilLog.Debug("Cache saved successfully", 
		"file", cacheFile, 
		"size_bytes", len(data))
}