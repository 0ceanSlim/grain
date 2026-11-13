package api

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/branding"
	"github.com/0ceanslim/grain/server/utils"
)

// grain_changerelaylogo / grain_changerelaylogolight: advertised, validated,
// empty clears, values kept exactly as set (like changerelayicon); only
// grain_storebrandingimage makes grain's own copy.
func TestNIP86ChangeRelayLogo(t *testing.T) {
	methods := map[string]bool{}
	for _, m := range supportedNIP86Methods() {
		methods[m] = true
	}
	for _, m := range []string{"grain_changerelaylogo", "grain_changerelaylogolight"} {
		if !methods[m] {
			t.Errorf("supportedmethods missing %q", m)
		}
	}

	dir := t.TempDir()
	p := filepath.Join(dir, "relay_metadata.json")
	if err := os.WriteFile(p, []byte(`{"name":"Test Relay"}`), 0644); err != nil {
		t.Fatal(err)
	}
	utils.SetRelayMetadataWritePath(p)
	branding.SetDir(filepath.Join(dir, "branding"))
	prevCfg := config.GetConfig()
	config.SetConfigForTesting(nil)
	t.Cleanup(func() {
		utils.SetRelayMetadataWritePath("relay_metadata.json")
		branding.SetDir("")
		config.SetConfigForTesting(prevCfg)
	})
	if err := utils.LoadRelayMetadata(p); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 40, 8))); err != nil {
		t.Fatal(err)
	}
	upload := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	for _, c := range []struct {
		method, value, wantErr, wantLogo, wantLight string
	}{
		{"grain_changerelaylogo", "javascript:alert(1)", "invalid logo URL", "", ""},
		{"grain_changerelaylogolight", "not a url", "invalid logo_light URL", "", ""},
		// Kept exactly as set: no fetch, no copy.
		{"grain_changerelaylogo", "https://cdn.example/logo.png", "", "https://cdn.example/logo.png", ""},
		{"grain_changerelaylogolight", "https://cdn.example/dark.png", "", "https://cdn.example/logo.png", "https://cdn.example/dark.png"},
		{"grain_changerelaylogolight", "", "", "https://cdn.example/logo.png", ""},
	} {
		_, errStr := dispatchNIP86(nip86Request{Method: c.method, Params: []any{c.value}}, "owner")
		if errStr != c.wantErr {
			t.Fatalf("%s(%q): error %q, want %q", c.method, c.value, errStr, c.wantErr)
		}
		m := utils.GetRelayMetadataCopy()
		if m.Logo != c.wantLogo || m.LogoLight != c.wantLight {
			t.Fatalf("%s(%q): logo %q light %q, want %q %q", c.method, c.value, m.Logo, m.LogoLight, c.wantLogo, c.wantLight)
		}
	}

	// An outside icon set through the plain method is kept as set (no fetch).
	if _, errStr := dispatchNIP86(nip86Request{Method: "changerelayicon", Params: []any{"https://cdn.example/icon.png"}}, "owner"); errStr != "" {
		t.Fatal(errStr)
	}
	if m := utils.GetRelayMetadataCopy(); m.Icon != "https://cdn.example/icon.png" {
		t.Fatalf("icon rewritten: %q", m.Icon)
	}
	// An upload goes through grain_storebrandingimage and is stored by grain.
	if _, errStr := dispatchNIP86(nip86Request{Method: "grain_storebrandingimage", Params: []any{"icon", upload}}, "owner"); errStr != "" {
		t.Fatal(errStr)
	}
	stored := utils.GetRelayMetadataCopy().Icon
	if !strings.HasPrefix(stored, "/branding/icon-") {
		t.Fatalf("uploaded icon not stored by grain: %q", stored)
	}
	// The plain methods take the relay-relative path the store wrote, so a
	// stored image can be reused in another field from the admin form.
	if _, errStr := dispatchNIP86(nip86Request{Method: "grain_changerelaylogo", Params: []any{stored}}, "owner"); errStr != "" {
		t.Fatalf("grain_changerelaylogo(%q): %s", stored, errStr)
	}
	if m := utils.GetRelayMetadataCopy(); m.Logo != stored {
		t.Fatalf("logo = %q, want the stored path %q", m.Logo, stored)
	}
	// The stored icon is pruned only once no field points at it: switching
	// the icon to an outside URL keeps it (the logo still uses it), clearing
	// the logo too removes it.
	storedFile := filepath.Join(branding.Dir(), strings.TrimPrefix(stored, branding.URLPrefix))
	if _, errStr := dispatchNIP86(nip86Request{Method: "changerelayicon", Params: []any{"https://cdn.example/icon.png"}}, "owner"); errStr != "" {
		t.Fatal(errStr)
	}
	if _, err := os.Stat(storedFile); err != nil {
		t.Fatalf("stored image pruned while the logo still uses it: %v", err)
	}
	if _, errStr := dispatchNIP86(nip86Request{Method: "grain_changerelaylogo", Params: []any{""}}, "owner"); errStr != "" {
		t.Fatal(errStr)
	}
	if _, err := os.Stat(storedFile); !os.IsNotExist(err) {
		t.Fatalf("stored image no field uses was kept: %v", err)
	}
	// A path that browsers read as another host is still refused.
	for _, v := range []string{"//evil.example/x.png", `/\evil.example/x.png`} {
		if _, errStr := dispatchNIP86(nip86Request{Method: "changerelaybanner", Params: []any{v}}, "owner"); errStr != "invalid banner URL" {
			t.Fatalf("changerelaybanner(%q): error %q, want invalid banner URL", v, errStr)
		}
	}
}
