package relay

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Strings json.Marshal would encode differently from NIP-01.
var nip01Strings = []string{
	"",
	"plain",
	"a & b",
	"a < b > c",
	"https://x.com/p?a=1&b=2",
	"a &amp; b",
	`say "hi"`,
	`back\slash`,
	`literal \u0026 text`,
	"line\nbreak\r\ttab\bback\fform",
	"ctrl \x01 \x1f del \x7f",
	"sep \u2028 \u2029",
	"café 日本語 🌾 ·",
}

func TestAppendNIP01String_Escapes(t *testing.T) {
	cases := map[string]string{
		"a & b":        `"a & b"`,
		"<>":           `"<>"`,
		"\u2028":       "\"\u2028\"",
		"\x01":         "\"\x01\"",
		`"\`:           `"\"\\"`,
		"\n\r\t\b\f":   `"\n\r\t\b\f"`,
		`\u0026`:       `"\\u0026"`,
		"日本語":          `"日本語"`,
		"end\\":        `"end\\"`,
		"\"start":      `"\"start"`,
		"mid\"dle\"\"": `"mid\"dle\"\""`,
	}
	for in, want := range cases {
		if got := string(appendNIP01String(nil, in)); got != want {
			t.Errorf("appendNIP01String(%q) = %q, want %q", in, got, want)
		}
	}
}

// Without control characters or U+2028/9, NIP-01 escaping equals
// encoding/json with HTML escaping off, which pins the array layout.
func TestCommitment_MatchesUnescapedEncoder(t *testing.T) {
	for _, s := range nip01Strings {
		if strings.ContainsAny(s, "\x01\x1f\u2028\u2029") {
			continue
		}
		evt := Event{
			PubKey:    strings.Repeat("ab", 32),
			CreatedAt: 1700000000,
			Kind:      1,
			Tags:      [][]string{{"r", s}, {"t", "x", s}},
			Content:   s,
		}
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode([]interface{}{0, evt.PubKey, evt.CreatedAt, evt.Kind, evt.Tags, evt.Content}); err != nil {
			t.Fatal(err)
		}
		want := strings.TrimSuffix(buf.String(), "\n")
		if got := string(evt.Commitment()); got != want {
			t.Errorf("content %q:\n got %s\nwant %s", s, got, want)
		}
	}
}

func TestCanonicalJSON_RoundTrips(t *testing.T) {
	for _, s := range nip01Strings {
		// NIP-01 keeps control characters raw. nostrdb's tokenizer takes
		// them; encoding/json's strict decoder does not, so skip them here.
		if strings.ContainsAny(s, "\x01\x1f") {
			continue
		}
		evt := Event{
			ID:        strings.Repeat("01", 32),
			PubKey:    strings.Repeat("ab", 32),
			CreatedAt: 1700000000,
			Kind:      30023,
			Tags:      [][]string{{"d", s}, {"imeta", "url " + s}},
			Content:   s,
			Sig:       strings.Repeat("cd", 64),
		}
		raw := evt.CanonicalJSON()
		if bytes.Contains(raw, []byte(`\u`)) && !strings.Contains(s, `\u`) {
			t.Errorf("content %q: output contains a \\u escape: %s", s, raw)
		}
		var back Event
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatalf("content %q: %v\n%s", s, err, raw)
		}
		if !reflect.DeepEqual(back, evt) {
			t.Errorf("content %q: round trip mismatch\n got %+v\nwant %+v", s, back, evt)
		}
	}
}

func TestCommitment_NilTagsAreEmptyArray(t *testing.T) {
	evt := Event{PubKey: "pk", CreatedAt: 1, Kind: 1, Content: "c"}
	if got, want := string(evt.Commitment()), `[0,"pk",1,1,[],"c"]`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if got := string(evt.CanonicalJSON()); !strings.Contains(got, `"tags":[]`) {
		t.Errorf("CanonicalJSON nil tags: %s", got)
	}
}
