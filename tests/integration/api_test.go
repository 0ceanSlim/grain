package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

func TestRelayInfoEndpoint(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequest("GET", tests.TestHTTPURL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Set NIP-11 accept header
	req.Header.Set("Accept", "application/nostr+json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/nostr+json" {
		t.Fatalf("Expected content-type 'application/nostr+json', got '%s'", contentType)
	}

	// Parse JSON response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var relayInfo map[string]interface{}
	err = json.Unmarshal(body, &relayInfo)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Check required NIP-11 fields
	requiredFields := []string{"name", "description", "supported_nips", "software", "version"}
	for _, field := range requiredFields {
		if _, exists := relayInfo[field]; !exists {
			t.Errorf("Missing required field: %s", field)
		}
	}

	t.Logf("✅ Relay info endpoint working correctly")
	t.Logf("📋 Relay name: %v", relayInfo["name"])
	t.Logf("📋 Supported NIPs: %v", relayInfo["supported_nips"])
}

func TestWebPageEndpoint(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(tests.TestHTTPURL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Should return HTML
	contentType := resp.Header.Get("Content-Type")
	if !containsString(contentType, "text/html") {
		t.Fatalf("Expected HTML content, got: %s", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	// Check for basic HTML structure
	bodyStr := string(body)
	if !containsString(bodyStr, "<html") || !containsString(bodyStr, "GRAIN") {
		t.Fatalf("Response doesn't look like GRAIN HTML page")
	}

	t.Log("✅ Web page endpoint working correctly")
}

func TestAPIEndpoints(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}

	testCases := []struct {
		endpoint string
		name     string
	}{
		{"/api/v1/whitelist/pubkeys", "whitelist"},
		{"/api/v1/blacklist/pubkeys", "blacklist"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := client.Get(tests.TestHTTPURL + tc.endpoint)
			if err != nil {
				t.Fatalf("Failed to make request to %s: %v", tc.endpoint, err)
			}
			defer resp.Body.Close()

			// Should return JSON (might be 404 if disabled, but should be valid response)
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
				t.Fatalf("Unexpected status for %s: %d", tc.endpoint, resp.StatusCode)
			}

			t.Logf("✅ %s endpoint responding (status: %d)", tc.name, resp.StatusCode)
		})
	}
}

// Helper function
func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle ||
			haystack[:len(needle)] == needle ||
			haystack[len(haystack)-len(needle):] == needle ||
			indexOfString(haystack, needle) >= 0)
}

func indexOfString(haystack, needle string) int {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
