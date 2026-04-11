package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
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

	req.Header.Set("Accept", "application/nostr+json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/nostr+json" {
		t.Fatalf("Expected content-type 'application/nostr+json', got '%s'", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	var relayInfo map[string]interface{}
	err = json.Unmarshal(body, &relayInfo)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	requiredFields := []string{"name", "description", "supported_nips", "software", "version"}
	for _, field := range requiredFields {
		if _, exists := relayInfo[field]; !exists {
			t.Errorf("Missing required NIP-11 field: %s", field)
		}
	}

	t.Logf("Relay info: name=%v, version=%v", relayInfo["name"], relayInfo["version"])
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

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Fatalf("Expected HTML content, got: %s", contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, "<html") {
		t.Fatal("Response doesn't contain HTML")
	}

	t.Log("Web page endpoint serving HTML correctly")
}

func TestAPIEndpoints(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}

	endpoints := []struct {
		path string
		name string
	}{
		{"/api/v1/whitelist/pubkeys", "whitelist"},
		{"/api/v1/blacklist/pubkeys", "blacklist"},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			resp, err := client.Get(tests.TestHTTPURL + ep.path)
			if err != nil {
				t.Fatalf("Request to %s failed: %v", ep.path, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
				t.Fatalf("Unexpected status for %s: %d", ep.path, resp.StatusCode)
			}

			t.Logf("%s endpoint responding (status: %d)", ep.name, resp.StatusCode)
		})
	}
}
