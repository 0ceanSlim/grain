package relay

import (
	"encoding/json"
	"strings"
	"testing"
)

func parse(t *testing.T, js string) (Filter, bool, error) {
	t.Helper()
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(js), &raw); err != nil {
		t.Fatalf("bad test JSON %s: %v", js, err)
	}
	return ParseFilter(raw)
}

// Every filter the rc4 production check saw widen into an unconstrained
// query (or fail with a generic error) must now be rejected up front.
func TestParseFilter_RejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"string kind":          `{"kinds":["1"]}`,
		"fractional kind":      `{"kinds":[1.5]}`,
		"negative kind":        `{"kinds":[-1]}`,
		"kind way too large":   `{"kinds":[1e12]}`,
		"kinds not array":      `{"kinds":1}`,
		"65-char author":       `{"authors":["` + strings.Repeat("0", 65) + `"]}`,
		"non-hex id":           `{"ids":["` + strings.Repeat("G", 64) + `"]}`,
		"non-hex short id":     `{"ids":["zz"]}`,
		"empty id":             `{"ids":[""]}`,
		"number in ids":        `{"ids":[1]}`,
		"ids not array":        `{"ids":"abcd"}`,
		"negative limit":       `{"limit":-1}`,
		"negative limit+kinds": `{"kinds":[7],"limit":-1}`,
		"string limit":         `{"limit":"10"}`,
		"string since":         `{"since":"1700000000"}`,
		"negative until":       `{"until":-5}`,
		"search not string":    `{"search":1}`,
		"tag not array":        `{"#e":"abc"}`,
		"tag with number":      `{"#t":["nostr",1]}`,
	}
	for name, js := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := parse(t, js); err == nil {
				t.Fatalf("%s: expected an error", js)
			}
		})
	}
}

// Prefix lookups are deliberate (glyphbyte.dev): 1–64 hex chars, odd
// lengths included, must keep parsing and be flagged for the NOTICE.
func TestParseFilter_PrefixesAccepted(t *testing.T) {
	full := strings.Repeat("ab", 32)
	for _, id := range []string{"a", "abc", "0123456789abcdef", full[:63]} {
		f, ok, err := parse(t, `{"ids":["`+id+`"]}`)
		if err != nil || !ok {
			t.Fatalf("ids[%q]: ok=%v err=%v", id, ok, err)
		}
		if !f.HasPrefixMatch() {
			t.Fatalf("ids[%q]: expected HasPrefixMatch", id)
		}
	}

	f, _, err := parse(t, `{"authors":["`+full+`"],"ids":["`+full+`"]}`)
	if err != nil {
		t.Fatal(err)
	}
	if f.HasPrefixMatch() {
		t.Fatal("full 64-char ids/authors must not count as a prefix match")
	}

	// uppercase hex is lowered to match stored ids
	f, _, err = parse(t, `{"authors":["ABCDEF"]}`)
	if err != nil || f.Authors[0] != "abcdef" {
		t.Fatalf("uppercase author: got %v, err %v", f.Authors, err)
	}
}

func TestParseFilter_EmptyListsMatchNothing(t *testing.T) {
	for _, js := range []string{`{"ids":[]}`, `{"authors":[]}`, `{"kinds":[]}`, `{"#p":[]}`} {
		_, ok, err := parse(t, js)
		if err != nil || ok {
			t.Fatalf("%s: want matchable=false, nil err; got ok=%v err=%v", js, ok, err)
		}
	}
}

func TestParseFilter_ValidFields(t *testing.T) {
	f, ok, err := parse(t, `{"kinds":[1,30023,99999],"since":1700000000.9,"until":1800000000,"limit":0,"search":"grain","#t":["nostr"],"unknown":true}`)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if len(f.Kinds) != 3 || f.Kinds[2] != 99999 {
		t.Errorf("kinds = %v", f.Kinds)
	}
	if f.Since == nil || f.Since.Unix() != 1700000000 {
		t.Errorf("since = %v (fractions truncate)", f.Since)
	}
	if f.Until == nil || f.Until.Unix() != 1800000000 {
		t.Errorf("until = %v", f.Until)
	}
	if f.Limit == nil || *f.Limit != 0 {
		t.Errorf("limit = %v", f.Limit)
	}
	if f.Search != "grain" || len(f.Tags["t"]) != 1 {
		t.Errorf("search/tags = %q %v", f.Search, f.Tags)
	}

	// Tags is always non-nil: COUNT adds a p-tag constraint to it.
	f, _, _ = parse(t, `{}`)
	if f.Tags == nil {
		t.Fatal("Tags must be non-nil")
	}
}
