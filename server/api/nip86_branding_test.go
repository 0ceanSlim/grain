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

// grain_storebrandingimage: an upload is checked, stored, served from the
// relay's origin, and the slot's field points at it; bad input is refused
// with nothing written.
func TestNIP86StoreBrandingImage(t *testing.T) {
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
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 512, 512))); err != nil {
		t.Fatal(err)
	}
	upload := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	svg := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))

	call := func(params ...any) (any, string) {
		return dispatchNIP86(nip86Request{Method: "grain_storebrandingimage", Params: params}, "owner")
	}
	for _, bad := range [][]any{{"favicon", upload}, {"icon"}, {"icon", ""}, {"icon", svg}, {"icon", "ftp://x/y.png"}} {
		if _, errStr := call(bad...); errStr == "" {
			t.Errorf("%v: accepted", bad[:1])
		}
	}
	if m := utils.GetRelayMetadataCopy(); m.Icon != "" {
		t.Fatalf("refused input wrote the icon: %q", m.Icon)
	}

	res, errStr := call("icon", upload)
	if errStr != "" {
		t.Fatal(errStr)
	}
	out := res.(map[string]any)
	u, _ := out["url"].(string)
	if !strings.HasPrefix(u, "/branding/icon-") || out["width"] != 512 || out["type"] != "image/png" {
		t.Fatalf("result %v", out)
	}
	if m := utils.GetRelayMetadataCopy(); m.Icon != u {
		t.Fatalf("icon field %q, want %q", m.Icon, u)
	}
	if _, err := os.Stat(filepath.Join(dir, "branding", strings.TrimPrefix(u, "/branding/"))); err != nil {
		t.Fatalf("stored file: %v", err)
	}
}
