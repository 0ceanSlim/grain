package core

import (
	"encoding/json"
	"testing"

	nostr "github.com/0ceanslim/grain/server/types"
)

// The critical guarantee: editing some fields must preserve every other content
// field and tag, and must dual-write each edit to both content and a tag.
func TestAssembleProfileEventPreservesAndDualWrites(t *testing.T) {
	existing := &nostr.Event{
		PubKey:  "abc",
		Kind:    0,
		Content: `{"name":"old","about":"my bio","custom_field":"keepme"}`,
		Tags:    [][]string{{"i", "github:alice", "proof"}, {"name", "old"}},
	}

	got, err := AssembleProfileEvent(existing, map[string]string{
		"name":         "new",
		"display_name": "New Name",
	})
	if err != nil {
		t.Fatal(err)
	}

	var c map[string]string
	if err := json.Unmarshal([]byte(got.Content), &c); err != nil {
		t.Fatalf("content not valid JSON: %v", err)
	}
	if c["name"] != "new" {
		t.Errorf("name = %q, want new", c["name"])
	}
	if c["display_name"] != "New Name" {
		t.Errorf("display_name = %q, want New Name", c["display_name"])
	}
	if c["about"] != "my bio" {
		t.Errorf("about not preserved: %q", c["about"])
	}
	if c["custom_field"] != "keepme" {
		t.Errorf("UNKNOWN FIELD LOST: custom_field = %q", c["custom_field"])
	}

	iCount, nameCount := 0, 0
	var nameVal, displayVal string
	for _, tag := range got.Tags {
		switch tag[0] {
		case "i":
			iCount++
		case "name":
			nameCount++
			nameVal = tag[1]
		case "display_name":
			displayVal = tag[1]
		}
	}
	if iCount != 1 {
		t.Errorf("NIP-39 i tag not preserved (count %d)", iCount)
	}
	if nameCount != 1 || nameVal != "new" {
		t.Errorf("name tag not upserted (replaced): count=%d val=%q", nameCount, nameVal)
	}
	if displayVal != "New Name" {
		t.Errorf("display_name tag missing")
	}

	if got.ID != "" || got.Sig != "" {
		t.Error("assembled event must be unsigned")
	}
	if got.PubKey != "abc" || got.Kind != 0 {
		t.Errorf("pubkey/kind wrong: %q/%d", got.PubKey, got.Kind)
	}
}

// The assembler is now policy-free about the client tag: it preserves whatever
// is there (the build handler applies ApplyClientTag to strip foreign + stamp
// grain's own). This documents that division and the end-to-end result.
func TestAssembleProfileEventClientTagHandledByHelper(t *testing.T) {
	existing := &nostr.Event{
		PubKey:  "abc",
		Content: `{"name":"alice"}`,
		Tags:    [][]string{{"client", "amethyst", "31990:..."}, {"i", "github:alice", "p"}},
	}
	got, err := AssembleProfileEvent(existing, map[string]string{"about": "hi"})
	if err != nil {
		t.Fatal(err)
	}
	// Assembler preserves the existing client tag (no longer strips it).
	if !hasTag(got.Tags, "client", "amethyst") {
		t.Errorf("assembler should preserve the client tag for the handler to rewrite, got %v", got.Tags)
	}
	if !hasTag(got.Tags, "i", "github:alice") {
		t.Error("non-client tags must be preserved")
	}

	// The handler's helper then strips the foreign tag and stamps grain's.
	ApplyClientTag(got, true, "grain")
	if hasTag(got.Tags, "client", "amethyst") {
		t.Error("ApplyClientTag should strip the foreign client tag")
	}
	if !hasTag(got.Tags, "client", "grain") || countTag(got.Tags, "client") != 1 {
		t.Errorf("ApplyClientTag should leave exactly one client:grain, got %v", got.Tags)
	}
}

func TestAssembleProfileEventNewProfile(t *testing.T) {
	got, err := AssembleProfileEvent(&nostr.Event{PubKey: "abc"}, map[string]string{"name": "alice"})
	if err != nil {
		t.Fatal(err)
	}
	var c map[string]string
	if err := json.Unmarshal([]byte(got.Content), &c); err != nil {
		t.Fatal(err)
	}
	if c["name"] != "alice" {
		t.Errorf("name not set on new profile: %q", c["name"])
	}
}

func TestAssembleProfileEventRejectsInvalidExistingContent(t *testing.T) {
	_, err := AssembleProfileEvent(&nostr.Event{PubKey: "abc", Content: "not json"}, map[string]string{"name": "x"})
	if err == nil {
		t.Fatal("expected an error for invalid existing content — must not clobber a profile we can't parse")
	}
}
