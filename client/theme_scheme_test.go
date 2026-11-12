package client

import (
	"os"
	"regexp"
	"testing"
)

// theme.js publishes each theme's scheme (<html data-color-scheme>) before
// any CSS loads, so the header can pick the relay's light-theme logo with no
// flash. The scheme there must match the theme's own `color-scheme` in
// input.css, which is the source of truth.
func TestThemeSchemesMatchStylesheet(t *testing.T) {
	css, err := os.ReadFile("../www/style/input.css")
	if err != nil {
		t.Fatal(err)
	}
	js, err := os.ReadFile("../www/static/js/theme.js")
	if err != nil {
		t.Fatal(err)
	}
	cssSchemes := map[string]string{}
	for _, m := range regexp.MustCompile(`(?s):root\[data-theme="([^"]+)"\]\s*\{[^}]*?color-scheme:\s*(light|dark)`).FindAllSubmatch(css, -1) {
		cssSchemes[string(m[1])] = string(m[2])
	}
	jsThemes := regexp.MustCompile(`\{\s*id:\s*"([^"]+)"[^}]*?scheme:\s*"(light|dark)"`).FindAllSubmatch(js, -1)
	if len(jsThemes) == 0 || len(cssSchemes) == 0 {
		t.Fatalf("found %d themes in theme.js, %d in input.css", len(jsThemes), len(cssSchemes))
	}
	for _, m := range jsThemes {
		id, scheme := string(m[1]), string(m[2])
		want, ok := cssSchemes[id]
		if !ok {
			t.Errorf("theme %q: no color-scheme in input.css", id)
			continue
		}
		if scheme != want {
			t.Errorf("theme %q: theme.js scheme %q, input.css color-scheme %q", id, scheme, want)
		}
	}
	if len(jsThemes) != len(cssSchemes) {
		t.Errorf("theme.js lists %d themes, input.css defines %d", len(jsThemes), len(cssSchemes))
	}
}
