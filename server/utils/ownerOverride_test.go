package utils

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// GRAIN_OWNER_PUBKEY is applied in memory only; a metadata write (which
// reloads relay_metadata.json, whose pubkey is empty) must not drop it, or
// the owner is locked out of every NIP-86 call after their first save.
func TestOwnerOverrideSurvivesMetadataWrites(t *testing.T) {
	p := filepath.Join(t.TempDir(), "relay_metadata.json")
	if err := os.WriteFile(p, []byte(`{"name":"Test Relay","pubkey":""}`), 0644); err != nil {
		t.Fatal(err)
	}
	prevPath := relayMetadataWritePath
	prevMeta := GetRelayMetadataCopy()
	prevOwner := envOwner()
	t.Cleanup(func() {
		SetRelayMetadataWritePath(prevPath)
		relayMetadata = prevMeta
		ownerOverride.Store(prevOwner)
	})
	SetRelayMetadataWritePath(p)
	if err := LoadRelayMetadata(p); err != nil {
		t.Fatal(err)
	}
	const owner = "d7befd3f0bca917db20aad40e08a624ce39c74ff95bcf665cd89960c30ad9efb"
	OverrideRelayOwnerInMemory(owner)

	if err := SetRelayMetadataField("icon", "https://example.com/i.png"); err != nil {
		t.Fatal(err)
	}
	name := "Renamed"
	if err := UpdateRelayMetadata(&name, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if got := GetRelayOwnerPubkey(); got != owner {
		t.Fatalf("owner after metadata writes = %q, want the env override", got)
	}
	if IsRelayUnowned() {
		t.Fatal("relay reads as unowned after a metadata write")
	}
	// Reading as unowned would also let /setup take a new claim.
	const other = "1111111111111111111111111111111111111111111111111111111111111111"
	if err := SetRelayOwner(other); !errors.Is(err, ErrOwnerAlreadySet) {
		t.Fatalf("SetRelayOwner after metadata writes = %v, want ErrOwnerAlreadySet", err)
	}
	if m := GetRelayMetadataCopy(); m.Pubkey != owner {
		t.Fatalf("NIP-11 pubkey after metadata writes = %q, want the env override", m.Pubkey)
	}
	if m := GetRelayMetadataCopy(); m.Icon != "https://example.com/i.png" || m.Name != "Renamed" {
		t.Fatalf("writes not applied: %+v", m)
	}
}
