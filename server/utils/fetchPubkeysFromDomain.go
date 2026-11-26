package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
	"github.com/0ceanslim/grain/server/utils/safehttp"
)

// NostrJSON represents the structure of .well-known/nostr.json
type NostrJSON struct {
	Names map[string]string `json:"names"`
}

// nostrJSONMaxBytes caps one domain's nostr.json. Large NIP-05 providers
// list tens of thousands of names in well under this.
const nostrJSONMaxBytes = 4 << 20

// nostrJSONClient fetches nostr.json from public addresses only and follows
// no redirects (NIP-05: fetchers MUST ignore redirects).
var nostrJSONClient = safehttp.NewClient(10*time.Second, 0)

// ValidDomain reports whether d is a plain DNS host name (labels of letters,
// digits and hyphens, at least one dot, a non-numeric top label): no port,
// no IP literal, no path. Domains from config and the admin preview both go
// through it before anything is fetched.
func ValidDomain(d string) bool {
	if len(d) == 0 || len(d) > 253 {
		return false
	}
	labels := strings.Split(d, ".")
	if len(labels) < 2 {
		return false
	}
	for _, l := range labels {
		if len(l) == 0 || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
			return false
		}
		for i := 0; i < len(l); i++ {
			c := l[i]
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	top := labels[len(labels)-1]
	allDigits := true
	for i := 0; i < len(top); i++ {
		if top[i] < '0' || top[i] > '9' {
			allDigits = false
		}
	}
	return !allDigits
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

// FetchDomainNames returns the parsed names map from a single
// domain's .well-known/nostr.json. Exported for the admin
// dashboard's domain-preview endpoint — the cache refresher uses
// fetchDomainPubkeys instead (which throws away the names).
func FetchDomainNames(domain string) (map[string]string, error) {
	pubkeys, names, err := fetchDomainBoth(domain)
	_ = pubkeys
	return names, err
}

// fetchDomainPubkeys fetches pubkeys from a single domain's .well-known/nostr.json
func fetchDomainPubkeys(domain string) ([]string, error) {
	pubkeys, _, err := fetchDomainBoth(domain)
	return pubkeys, err
}

// fetchDomainBoth does the actual HTTP fetch + parse, returning
// both the flat pubkey list (for the cache refresher) and the
// name→pubkey map (for the admin dashboard's per-domain preview).
// One implementation; two callers with different needs.
func fetchDomainBoth(domain string) ([]string, map[string]string, error) {
	if !ValidDomain(domain) {
		return nil, nil, fmt.Errorf("not a domain name: %q", domain)
	}
	url := fmt.Sprintf("https://%s/.well-known/nostr.json", domain)

	log.Util().Debug("Fetching nostr.json",
		"domain", domain,
		"url", url)

	resp, err := nostrJSONClient.Get(url)
	if err != nil {
		return nil, nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, nostrJSONMaxBytes+1))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if len(body) > nostrJSONMaxBytes {
		return nil, nil, fmt.Errorf("nostr.json is over %d bytes", nostrJSONMaxBytes)
	}

	// Parse JSON
	var nostrData NostrJSON
	if err := json.Unmarshal(body, &nostrData); err != nil {
		return nil, nil, fmt.Errorf("failed to parse JSON (size: %d bytes): %w", len(body), err)
	}

	// Keep only well-formed pubkeys: the names map is served to the
	// dashboard and the pubkeys feed the whitelist.
	var pubkeys []string
	names := make(map[string]string, len(nostrData.Names))
	for name, pubkey := range nostrData.Names {
		if !IsHexPubkey(pubkey) {
			continue
		}
		names[name] = pubkey
		pubkeys = append(pubkeys, pubkey)
		log.Util().Debug("Found pubkey in domain",
			"domain", domain,
			"name", name,
			"pubkey", pubkey)
	}

	if len(pubkeys) == 0 {
		log.Util().Warn("No valid pubkeys found in domain",
			"domain", domain,
			"names_count", len(nostrData.Names))
	}

	return pubkeys, names, nil
}
