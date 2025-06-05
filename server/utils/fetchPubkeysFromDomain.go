package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
)

// NostrJSON represents the structure of .well-known/nostr.json
type NostrJSON struct {
	Names map[string]string `json:"names"`
}

// FetchPubkeysFromDomains fetches nostr.json pubkeys from multiple domains
// This function is called by the pubkey cache system and doesn't maintain its own cache
func FetchPubkeysFromDomains(domains []string) ([]string, error) {
	log.Util().Info("Fetching pubkeys from domains", "domain_count", len(domains))
	
	if len(domains) == 0 {
		log.Util().Debug("No domains provided for pubkey fetching")
		return []string{}, nil
	}

	var allPubkeys []string
	successCount := 0
	errorCount := 0

	// Process each domain
	for _, domain := range domains {
		domainPubkeys, err := fetchDomainPubkeys(domain)
		if err != nil {
			log.Util().Warn("Failed to fetch pubkeys from domain", 
				"domain", domain, 
				"error", err)
			errorCount++
			continue
		}

		allPubkeys = append(allPubkeys, domainPubkeys...)
		successCount++
		
		log.Util().Info("Successfully fetched pubkeys from domain", 
			"domain", domain, 
			"pubkey_count", len(domainPubkeys))
	}

	log.Util().Info("Domain pubkey fetch completed", 
		"total_domains", len(domains),
		"successful_domains", successCount,
		"failed_domains", errorCount,
		"total_pubkeys", len(allPubkeys))

	return allPubkeys, nil
}

// fetchDomainPubkeys fetches pubkeys from a single domain's .well-known/nostr.json
func fetchDomainPubkeys(domain string) ([]string, error) {
	url := fmt.Sprintf("https://%s/.well-known/nostr.json", domain)
	
	log.Util().Debug("Fetching nostr.json", 
		"domain", domain, 
		"url", url)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make HTTP request
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse JSON
	var nostrData NostrJSON
	if err := json.Unmarshal(body, &nostrData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON (size: %d bytes): %w", len(body), err)
	}

	// Extract pubkeys
	var pubkeys []string
	for name, pubkey := range nostrData.Names {
		if pubkey != "" {
			pubkeys = append(pubkeys, pubkey)
			log.Util().Debug("Found pubkey in domain", 
				"domain", domain, 
				"name", name, 
				"pubkey", pubkey)
		}
	}

	if len(pubkeys) == 0 {
		log.Util().Warn("No valid pubkeys found in domain", 
			"domain", domain,
			"names_count", len(nostrData.Names))
	}

	return pubkeys, nil
}