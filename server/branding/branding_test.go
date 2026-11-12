package branding

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{255, 220, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCheckAcceptsRasterAndRefusesTheRest(t *testing.T) {
	img, err := Check(pngBytes(t, 64, 32))
	if err != nil || img.Type != "image/png" || img.Width != 64 || img.Height != 32 {
		t.Fatalf("png: %+v %v", img, err)
	}
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	for name, data := range map[string][]byte{
		"empty":     nil,
		"svg":       svg,
		"html":      []byte("<!doctype html><html></html>"),
		"too big":   append(pngBytes(t, 1, 1), make([]byte, MaxImageBytes)...),
		"truncated": pngBytes(t, 8, 8)[:30],
	} {
		if _, err := Check(data); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := Check(pngBytes(t, MaxSide+1, 1)); err == nil {
		t.Error("an image wider than MaxSide was accepted")
	}
}

func TestDecodeDataURL(t *testing.T) {
	raw := pngBytes(t, 2, 2)
	got, err := DecodeDataURL("data:image/png;base64," + base64.StdEncoding.EncodeToString(raw))
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("decode: %v", err)
	}
	for _, bad := range []string{"https://x/y.png", "data:image/png,rawbytes", "data:image/png;base64,!!!"} {
		if _, err := DecodeDataURL(bad); err == nil {
			t.Errorf("%q: accepted", bad)
		}
	}
}

// Save stores a content-addressed copy, points the field at its relative
// path, drops the slot's older copy only, and the copy is served with headers
// that keep it inert.
func TestSaveServeAndPrune(t *testing.T) {
	SetDir(t.TempDir())
	t.Cleanup(func() { SetDir("") })
	fields := map[string]string{}
	set := func(f, v string) error { fields[f] = v; return nil }

	a, _ := Check(pngBytes(t, 4, 4))
	b, _ := Check(pngBytes(t, 5, 5))
	u1, err := Save("logo", a, set)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Save("icon", b, set); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u1, "/branding/logo-") || !strings.HasSuffix(u1, ".png") || fields["logo"] != u1 {
		t.Fatalf("path %q fields %v", u1, fields)
	}
	u2, err := Save("logo", b, set)
	if err != nil || u2 == u1 {
		t.Fatalf("replace: %q %v", u2, err)
	}
	names, _ := filepath.Glob(filepath.Join(Dir(), "*"))
	if len(names) != 2 { // logo (new) + icon; the old logo is pruned
		t.Fatalf("files after replace: %v", names)
	}

	rec := httptest.NewRecorder()
	ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u2, nil))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" || rec.Header().Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(rec.Header().Get("Content-Security-Policy"), "sandbox") || !bytes.Equal(rec.Body.Bytes(), b.Data) {
		t.Fatalf("serve: %d %v", rec.Code, rec.Header())
	}
	for _, p := range []string{u1, "/branding/../relay_metadata.json", "/branding/x.png", "/branding/"} {
		rec := httptest.NewRecorder()
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: HTTP %d, want 404", p, rec.Code)
		}
	}
	rec = httptest.NewRecorder()
	ServeHTTP(rec, httptest.NewRequest(http.MethodPost, u2, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: HTTP %d", rec.Code)
	}
}

func TestPublicBaseAbsoluteAndLocalPath(t *testing.T) {
	for in, want := range map[string]string{
		"wss://relay.example":     "https://relay.example",
		"ws://localhost:8181/":    "http://localhost:8181",
		"https://relay.example/x": "https://relay.example",
	} {
		if got := PublicBase(in, nil); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	r.Host = "relay.example"
	r.Header.Set("X-Forwarded-Proto", "https")
	if got := PublicBase("", r); got != "https://relay.example" {
		t.Errorf("from request: %q", got)
	}
	if got := PublicBase("", nil); got != "" {
		t.Errorf("nothing known: %q", got)
	}
	if got := Absolute("/branding/icon-0123456789abcdef.png", "https://relay.example/"); got != "https://relay.example/branding/icon-0123456789abcdef.png" {
		t.Errorf("absolute: %q", got)
	}
	for _, v := range []string{"https://cdn.example/a.png", "//cdn.example/a.png", ""} {
		if got := Absolute(v, "https://relay.example"); got != v {
			t.Errorf("Absolute(%q) changed it to %q", v, got)
		}
	}
	for v, want := range map[string]bool{
		"/static/icons/x.png":                 true,
		"/branding/icon-0123456789abcdef.png": true,
		"//cdn.example/a.png":                 false, // scheme-relative: another host
		"/\\cdn.example/a.png":                false, // browsers read as //cdn.example
		"/a\nb":                               false,
		"https://relay.example/static/x.png":  false,
		"javascript:alert(1)":                 false,
		"":                                    false,
	} {
		if got := IsLocalPath(v); got != want {
			t.Errorf("IsLocalPath(%q) = %v, want %v", v, got, want)
		}
	}
}

func TestPublicOnlyRefusesNonPublicRanges(t *testing.T) {
	for _, a := range []string{"127.0.0.1", "10.1.2.3", "192.168.1.1", "169.254.169.254", "100.100.100.100", "198.18.0.1",
		"192.0.0.8", "240.0.0.1", "0.1.2.3", "::1", "fd00::1", "64:ff9b::a00:1", "2002:a00:1::1", "::ffff:127.0.0.1"} {
		if err := publicOnly("tcp", net.JoinHostPort(a, "443"), nil); err == nil {
			t.Errorf("%s: allowed", a)
		}
	}
	for _, a := range []string{"93.184.216.34", "2606:4700::6810:84e5"} {
		if err := publicOnly("tcp", net.JoinHostPort(a, "443"), nil); err != nil {
			t.Errorf("%s: refused: %v", a, err)
		}
	}
}

// Fetch refuses non-public addresses unless a test allows them, and
// non-http(s) URLs; with them allowed, an image comes back checked.
func TestFetch(t *testing.T) {
	raw := pngBytes(t, 6, 3)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/logo.png" {
			w.Write(raw)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	if _, err := Fetch(context.Background(), srv.URL+"/logo.png"); err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("loopback fetch should be refused, got %v", err)
	}
	if _, err := Fetch(context.Background(), "file:///etc/passwd"); err == nil {
		t.Fatal("file: URL accepted")
	}

	allowPrivate = true
	t.Cleanup(func() { allowPrivate = false })
	img, err := Fetch(context.Background(), srv.URL+"/logo.png")
	if err != nil || img.Type != "image/png" || img.Width != 6 || img.Height != 3 {
		t.Fatalf("fetch: %+v %v", img, err)
	}
	if _, err := Fetch(context.Background(), srv.URL+"/missing.png"); err == nil {
		t.Fatal("a 404 was accepted")
	}
}
