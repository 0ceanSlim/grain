package client

import (
	"bytes"
	"html/template"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/0ceanslim/grain/server/branding"
)

var h1RE = regexp.MustCompile(`(?s)<h1\b.*?</h1>`)

// renderApp renders the homepage shell (layout + header) with branding b
// and returns the whole page and its header <h1>.
func renderApp(t *testing.T, b RelayBranding) (page, h1 string) {
	t.Helper()
	prevFS, prevSrc := wwwFS, relayBranding
	t.Cleanup(func() { wwwFS, relayBranding = prevFS, prevSrc })
	SetEmbeddedWWW(os.DirFS(".."))
	SetRelayBranding(func() RelayBranding { return b })
	rec := httptest.NewRecorder()
	RenderTemplate(rec, PageData{Title: RelayPageTitle()}, "app.html")
	page = rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("render: HTTP %d: %s", rec.Code, page)
	}
	h1 = h1RE.FindString(page)
	if h1 == "" {
		t.Fatalf("no <h1> in page")
	}
	return page, h1
}

func TestBrandingNothingSetIsGrain(t *testing.T) {
	page, h1 := renderApp(t, RelayBranding{})
	if !strings.Contains(h1, "🌾 grain") || strings.Contains(h1, "<img") || !strings.Contains(h1, "shrink-0") {
		t.Errorf("header should be grain's wheat logo: %s", h1)
	}
	for _, want := range []string{`<title>🌾 grain</title>`, `href="/favicon.ico"`, `/static/icons/icon-180x180.png`, `name="apple-mobile-web-app-title" content="Grain"`} {
		if !strings.Contains(page, want) {
			t.Errorf("page missing %q", want)
		}
	}
}

func TestBrandingNameOnlyShowsName(t *testing.T) {
	page, h1 := renderApp(t, RelayBranding{Name: "  Test Relay "})
	if !strings.Contains(h1, `<span class="block truncate">Test Relay</span>`) || strings.Contains(h1, "grain") {
		t.Errorf("header should be the relay name: %s", h1)
	}
	if !strings.Contains(page, `<title>Test Relay</title>`) || !strings.Contains(page, `href="/favicon.ico"`) {
		t.Error("title should be the name; favicon stays grain's without an icon")
	}
}

func TestBrandingLogoShowsAloneWithIconAsFavicon(t *testing.T) {
	page, h1 := renderApp(t, RelayBranding{Name: "Test Relay", Icon: "https://example.com/i.png", Logo: "https://example.com/logo.png"})
	if strings.Count(h1, "<img") != 1 || !strings.Contains(h1, `src="https://example.com/logo.png"`) || !strings.Contains(h1, `alt="Test Relay"`) {
		t.Errorf("header should be the logo image: %s", h1)
	}
	if strings.Contains(h1, ">Test Relay<") || strings.Contains(h1, "data-logo-scheme") {
		t.Errorf("logo shows alone, with no light variant: %s", h1)
	}
	for _, want := range []string{`<link rel="icon" href="https://example.com/i.png" />`, `<link rel="apple-touch-icon" href="https://example.com/i.png" />`, `name="apple-mobile-web-app-title" content="Test Relay"`, `<title>Test Relay</title>`} {
		if !strings.Contains(page, want) {
			t.Errorf("page missing %q", want)
		}
	}
	if strings.Contains(page, "/favicon.ico") || strings.Contains(page, "icon-32x32") {
		t.Error("grain's favicons must give way to the relay icon")
	}
}

func TestBrandingLightLogoRendersBothVariants(t *testing.T) {
	_, h1 := renderApp(t, RelayBranding{Name: "Test Relay", Logo: "https://example.com/logo.png", LogoLight: "https://example.com/dark.png"})
	if !strings.Contains(h1, `data-logo-scheme="dark"`) || !strings.Contains(h1, `data-logo-scheme="light"`) || !strings.Contains(h1, `src="https://example.com/dark.png"`) {
		t.Errorf("both logo variants should render, tagged by scheme: %s", h1)
	}
	// A light variant without a logo does nothing (it is a Logo variant).
	_, h1 = renderApp(t, RelayBranding{Name: "Test Relay", LogoLight: "https://example.com/dark.png"})
	if strings.Contains(h1, "<img") {
		t.Errorf("logo_light alone should not render: %s", h1)
	}
}

func TestManifestFollowsRelayIcon(t *testing.T) {
	grainIcon := PWAIcon{Src: "/static/icons/icon-512x512.png", Sizes: "512x512", Type: "image/png"}
	m := PWAManifest{Name: "Grain - Nostr Relay", ShortName: "Grain", Icons: []PWAIcon{grainIcon}}
	applyRelayBranding(&m, RelayBranding{Name: "Test Relay"})
	if m.Name != "Grain - Nostr Relay" || len(m.Icons) != 1 {
		t.Fatalf("no icon: manifest must stay grain's, got %+v", m)
	}
	// An outside icon's size is unknown, so it is listed without one.
	applyRelayBranding(&m, RelayBranding{Name: "Test Relay", Icon: "https://example.com/i.png?v=2"})
	if m.Name != "Test Relay" || m.ShortName != "Test Relay" || len(m.Icons) != 2 {
		t.Fatalf("icon set: manifest should brand, got %+v", m)
	}
	if ic := m.Icons[0]; ic != (PWAIcon{Src: "https://example.com/i.png?v=2", Type: "image/png", Purpose: "any"}) {
		t.Errorf("icon entry %+v", ic)
	}
	if m.Icons[1] != grainIcon {
		t.Errorf("grain's icons must stay listed after the relay's: %+v", m.Icons)
	}
	// A copy grain stored is listed at its real size.
	branding.SetDir(t.TempDir())
	t.Cleanup(func() { branding.SetDir("") })
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 300, 300))); err != nil {
		t.Fatal(err)
	}
	img, err := branding.Check(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	name, err := branding.Store("icon", img)
	if err != nil {
		t.Fatal(err)
	}
	m = PWAManifest{Name: "Grain - Nostr Relay", Icons: []PWAIcon{grainIcon}}
	applyRelayBranding(&m, RelayBranding{Name: "Test Relay", Icon: branding.URLPrefix + name})
	if ic := m.Icons[0]; ic.Sizes != "300x300" || ic.Type != "image/png" {
		t.Errorf("stored icon entry %+v, want 300x300 image/png", ic)
	}
	// An icon type install prompts may not take (ICO, JPEG, GIF, unknown)
	// brands the name only; grain's icons stay.
	m = PWAManifest{Name: "Grain - Nostr Relay", Icons: []PWAIcon{grainIcon}}
	applyRelayBranding(&m, RelayBranding{Name: "Test Relay", Icon: "/branding/icon-0123456789abcdef.ico"})
	if m.Name != "Test Relay" || len(m.Icons) != 1 {
		t.Fatalf("ico icon: got %+v", m)
	}
	if got := iconMIMEType("https://example.com/icon"); got != "" {
		t.Errorf("unknown extension should omit type, got %q", got)
	}
}

// Image values a page can't load (a data: URL, which html/template renders
// as "#ZgotmplZ", a scheme-relative or backslash path, junk) are dropped, so
// the page keeps grain's own image instead of a broken one.
func TestBrandingDropsImagesPagesCannotLoad(t *testing.T) {
	for _, v := range []string{"data:image/png;base64,AAAA", "//cdn.example/i.png", "/\\cdn.example/i.png", "javascript:alert(1)", "not a url"} {
		page, h1 := renderApp(t, RelayBranding{Icon: v, Logo: v})
		if strings.Contains(h1, "<img") || !strings.Contains(page, `href="/favicon.ico"`) || strings.Contains(page, "ZgotmplZ") {
			t.Errorf("%q: should fall back to grain's images", v)
		}
	}
	for _, v := range []string{"/branding/icon-0123456789abcdef.png", "https://example.com/i.png"} {
		if got := webImageURL(v); got != v {
			t.Errorf("webImageURL(%q) = %q", v, got)
		}
	}
}

// The admin page parses the shared layout + header too, with its own func
// map; it must know every branding func they call.
func TestAdminTemplatesParseWithBrandingFuncs(t *testing.T) {
	prevFS := wwwFS
	t.Cleanup(func() { wwwFS = prevFS })
	SetEmbeddedWWW(os.DirFS(".."))
	patterns := append(layoutPatterns(), "www/views/admin.html", "www/views/components/*.html", "www/views/admin-sections/*.html")
	if _, err := template.New("").Funcs(adminTemplateFuncs).ParseFS(wwwFS, patterns...); err != nil {
		t.Fatal(err)
	}
}

// The setup pages parse with the base func map and use relayName too.
func TestSetupTemplatesParseWithBrandingFuncs(t *testing.T) {
	prevFS := wwwFS
	t.Cleanup(func() { wwwFS = prevFS })
	SetEmbeddedWWW(os.DirFS(".."))
	for _, v := range []string{"setup.html", "setup-claimed.html"} {
		patterns := append(layoutPatterns(), "www/views/"+v)
		if _, err := template.New("").Funcs(baseTemplateFuncs).ParseFS(wwwFS, patterns...); err != nil {
			t.Fatalf("%s: %v", v, err)
		}
	}
}
