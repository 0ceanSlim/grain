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

// Ids from nostr-tools getEventHash for content "a<c>b" and tag ["t","x<c>"],
// c = 0x00..0x1f. JSON.stringify writes the 27 control characters without a
// short escape as lowercase \u00XX, and so do go-nostr and rust-nostr.
var controlCharIDs = [32]string{
	"fe4fd46caaa62c258a16c555b7c4e404baf4a44b12699b4bd808a1b2525af719", // 0x00
	"0251bdb67b2e01c44aea6142c7ba3a8bf8d4a658cb5e8bd5295638bda902142e", // 0x01
	"79f6369d8c1c96a014b05bd6d85d037b55e2f40de34a0cce3ecb37861c7a46ba", // 0x02
	"d533e1b61f8bdfadf814d9e9aab2056b81b5406f6525324ab654487109a76ce2", // 0x03
	"5ea046dbc835543e771980910660c58a00d949ad46ec97f76be8e02f2a986eac", // 0x04
	"4dd254f8968be5f68ef429b36404cc39c86372b405d53ad42f851924b89b96ff", // 0x05
	"1268f27aed58be3176094acdd7de111e61e26af49ac1f0412e540afd5469a142", // 0x06
	"17a28df3420b6ab92963f5bababbfd6d305ae47e9993866497d851d5d2833387", // 0x07
	"d2ee05fbf0775f4b35d06fdcde8c87d1e97c0ed60a30b9a25b3baa57e8abe972", // 0x08
	"854090398d45f048c7bdb5b3edd0a13ba1858814e06026257fa58d5b437222e5", // 0x09
	"77db1012878c158128554473d422b8390f1e47191677949be4cbd83bc37d2774", // 0x0a
	"474ad20209c675534c1297abb62f7a4a7ea76f6b495a20d3d0ed614ab5423038", // 0x0b
	"1fb6c44d754ccbf4491b09edd567330ea7960ebc18b691c54525f2c83caaa379", // 0x0c
	"d35975a99336cce718d08d3e3f20138f4c06c53adbf5ed7dd72489521228d9ef", // 0x0d
	"15df6a2e26dd6bc0b1462b5d4fc09269b3d1342eb80a66f9c1b27be841332512", // 0x0e
	"eca820c35ed622c25399b68b2a1db4ac415b61335fb492d087a9543bb5012174", // 0x0f
	"636bb673857351d9b12938258ab0b75589064abc984e11c1f58e8270168f2989", // 0x10
	"6225dc7ae97ef5c398d7fc315e78cd6bbeea5d85bd25c422d677a723b9adccc0", // 0x11
	"b43c20bf5f660fa02d3f07e0dbfba8334f5792002b8799e40b3101bae43bfe1d", // 0x12
	"b9feebc49cdbab3a120b2f47413622323b62f968c56606ff0d8f5b7ffa4cbae2", // 0x13
	"8e3135ec03144be68c7ca0c9bd0f2e4999810ac0beb7e5e461454a85b36b5299", // 0x14
	"4045d0eecd27c2d3a9b3050ce044d03e88824662fbc0dd6c04dd6d209b6e1260", // 0x15
	"65ac00f567460dbd63e347299f1d0ceee1430b1bd939fb16995cc4fc3a2fa56f", // 0x16
	"ff2da4c6fd5baf0b123ac4424c602502bb00cb01cc5f68556224c76e17ebd032", // 0x17
	"65eb1468599c705cbec503069ada9ab8131c4f28cc0f51ed8368b190eca02e08", // 0x18
	"d939efeba4af6b6b54dcd28c392591c4c1f1c15651c4fef51570743e06f7a575", // 0x19
	"a53bffa52ba71bb1a218f1f4ca34fd6f0698b54f2a0f8ba1720ee3b15bb76692", // 0x1a
	"e5a51d6e896208023021bfda0f607c4de4e15776858f8df4b3459d4467c0cc9e", // 0x1b
	"33e2c927f330ae274bc05492c61cc9d2b6e4c91d0fb2010d4dc790134cc9538f", // 0x1c
	"02965f1d55898806485f5b2a159341be76aec87e17878357344ee0b1c5307b45", // 0x1d
	"99d500a3337ad6f2abaf4f51bf1f7fa7e00a22e4b5ca396102ce3d3a8b6a060f", // 0x1e
	"0fd4d225e8889aad2f8a34e0bc57a730ba11a429dde901f1004e922dbec0ce4c", // 0x1f
}

func TestComputeEventID_ControlCharsMatchNostrTools(t *testing.T) {
	for c, want := range controlCharIDs {
		ch := string(rune(c))
		evt := nostr.Event{
			PubKey:    strings.Repeat("ab", 32),
			CreatedAt: 1700000000,
			Kind:      1,
			Tags:      [][]string{{"t", "x" + ch}},
			Content:   "a" + ch + "b",
		}
		if got, _ := ComputeEventID(&evt); got != want {
			t.Errorf("control char 0x%02x: id %s, want %s (preimage %q)", c, got, want, SerializeEvent(evt))
		}
	}
}
