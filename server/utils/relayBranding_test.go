package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// logo / logo_light are grain's own web-page settings: they round-trip
// through relay_metadata.json (the same write path as the NIP-11 identity
// fields) but the NIP-11 document never carries them.
func TestRelayLogoStoredButNotServedOverNIP11(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "relay_metadata.json")
	if err := os.WriteFile(p, []byte(`{"name":"Test Relay","icon":"https://example.com/i.png","custom_ext":"kept"}`), 0644); err != nil {
		t.Fatal(err)
	}
	prevPath := relayMetadataWritePath
	prevMeta := GetRelayMetadataCopy()
	t.Cleanup(func() {
		SetRelayMetadataWritePath(prevPath)
		metadataMu.Lock()
		relayMetadata = prevMeta
		metadataMu.Unlock()
	})
	SetRelayMetadataWritePath(p)
	if err := LoadRelayMetadata(p); err != nil {
		t.Fatal(err)
	}
	if err := SetRelayMetadataField("logo", "https://example.com/logo.png"); err != nil {
		t.Fatal(err)
	}
	if err := SetRelayMetadataField("logo_light", "https://example.com/logo-dark.png"); err != nil {
		t.Fatal(err)
	}

	m := GetRelayMetadataCopy()
	if m.Logo != "https://example.com/logo.png" || m.LogoLight != "https://example.com/logo-dark.png" {
		t.Fatalf("in-memory logo not reloaded: %q %q", m.Logo, m.LogoLight)
	}
	raw, _ := os.ReadFile(p)
	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk["logo"] != "https://example.com/logo.png" || onDisk["custom_ext"] != "kept" {
		t.Fatalf("file not patched in place: %s", raw)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept", "application/nostr+json")
	rec := httptest.NewRecorder()
	RelayInfoHandler(rec, req)
	var doc map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("NIP-11 body: %v: %s", err, rec.Body.String())
	}
	for _, k := range []string{"logo", "logo_light"} {
		if _, ok := doc[k]; ok {
			t.Errorf("NIP-11 document must not carry %q: %s", k, rec.Body.String())
		}
	}
	if doc["icon"] != "https://example.com/i.png" || doc["name"] != "Test Relay" {
		t.Errorf("NIP-11 identity fields missing: %s", rec.Body.String())
	}
	// The serve path clears its copy only, never the live metadata.
	if GetRelayMetadataCopy().Logo == "" {
		t.Error("serving NIP-11 cleared the in-memory logo")
	}

	// grain's stored copies are relay-relative; NIP-11 serves them absolute.
	if err := SetRelayMetadataField("icon", "/branding/icon-0123456789abcdef.png"); err != nil {
		t.Fatal(err)
	}
	if err := SetRelayMetadataField("banner", "https://cdn.example/b.png"); err != nil {
		t.Fatal(err)
	}
	prevBase := PublicBaseProvider
	PublicBaseProvider = func(*http.Request) string { return "https://relay.example" }
	t.Cleanup(func() { PublicBaseProvider = prevBase })
	rec = httptest.NewRecorder()
	RelayInfoHandler(rec, req)
	doc = map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["icon"] != "https://relay.example/branding/icon-0123456789abcdef.png" || doc["banner"] != "https://cdn.example/b.png" {
		t.Errorf("NIP-11 icon/banner: %v %v", doc["icon"], doc["banner"])
	}
	if GetRelayMetadataCopy().Icon != "/branding/icon-0123456789abcdef.png" {
		t.Error("serving NIP-11 rewrote the stored icon")
	}
}
