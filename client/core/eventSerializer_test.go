package core

import (
	"strings"
	"testing"

	nostr "github.com/0ceanslim/grain/server/types"
)

// Ids from nostr-tools getEventHash for the same events. json.Marshal
// \u-escapes &, <, > and U+2028/9, so a serializer built on it gets the
// first and third wrong.
func TestComputeEventID_MatchesNostrTools(t *testing.T) {
	bs := `\`
	cases := []struct {
		name    string
		content string
		tags    [][]string
		want    string
	}{
		{"amp", "a & b < c > d", [][]string{{"r", "https://x.com/p?a=1&b=2"}},
			"b5131536e259c4c1444394eed09cd88fc5a70c79f5683674dc4b83b5c634ba36"},
		{"escapes", `say "hi" ` + bs + " back\nline\ttab", [][]string{},
			"d9eda133e11b3b8c9b1c39dc51578e4d2d1ccb2d1462f3d9cddaa4fa838b49da"},
		{"seps", "sep " + string(rune(0x2028)) + " " + string(rune(0x2029)) + " café 🌾", [][]string{{"t", "<tag>"}},
			"975da0b04b5d9ce368cb43116fd06f7e7dbba8ef2fb20a9394b710e3e795fa43"},
		{"literal-u", "literal " + bs + "u0026 text", [][]string{},
			"02bbd10847d3346902b90a4990ab2cdf82e623b8e0292d3be2e2a8c35178677d"},
	}
	for _, tc := range cases {
		evt := nostr.Event{
			PubKey:    strings.Repeat("ab", 32),
			CreatedAt: 1700000000,
			Kind:      1,
			Tags:      tc.tags,
			Content:   tc.content,
		}
		got, err := ComputeEventID(&evt)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s: id %s, want %s (preimage %s)", tc.name, got, tc.want, SerializeEvent(evt))
		}
	}
}
