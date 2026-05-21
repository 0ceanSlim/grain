package config

import (
	"strings"
	"testing"
)

const (
	pkA = "1111111111111111111111111111111111111111111111111111111111111111"
	pkB = "2222222222222222222222222222222222222222222222222222222222222222"
	pkC = "3333333333333333333333333333333333333333333333333333333333333333"
)

// withDataDir points the sidecar at a temp dir and resets in-memory state,
// restoring both when the test ends.
func withDataDir(t *testing.T) {
	t.Helper()
	prev := GetDataDir()
	SetDataDir(t.TempDir())
	ResetAdminMutelistForTest()
	t.Cleanup(func() {
		SetDataDir(prev)
		ResetAdminMutelistForTest()
	})
}

func TestNormalizePubkeySet(t *testing.T) {
	got := normalizePubkeySet([]string{
		"  " + strings.ToUpper(pkB) + "  ", // upper + whitespace → normalized
		pkA,
		pkA,       // duplicate dropped
		"not-hex", // invalid dropped
		"abc",     // wrong length dropped
		pkC,
	})
	want := []string{pkA, pkB, pkC} // sorted, deduped, lowercased
	if len(got) != len(want) {
		t.Fatalf("got %d pubkeys, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSetAdminMutelist_RoundTripAndUnion(t *testing.T) {
	withDataDir(t)

	if _, err := SetAdminMutelist(pkA, []string{pkB, pkC}); err != nil {
		t.Fatalf("SetAdminMutelist: %v", err)
	}

	// Persisted to disk: a fresh load should see the same set.
	ResetAdminMutelistForTest()
	LoadAdminMutelist()

	union := AdminMutelistPubkeys()
	if len(union) != 2 {
		t.Fatalf("after reload: got %d pubkeys, want 2: %v", len(union), union)
	}

	meta := GetAdminMutelistMeta()
	if len(meta) != 1 || meta[0].Pubkey != pkA || meta[0].Count != 2 {
		t.Fatalf("meta after reload: %+v", meta)
	}
	if meta[0].SyncedAt == 0 {
		t.Error("synced_at should be set")
	}
}

func TestSetAdminMutelist_EmptyClears(t *testing.T) {
	withDataDir(t)

	if _, err := SetAdminMutelist(pkA, []string{pkB}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if got := len(AdminMutelistPubkeys()); got != 1 {
		t.Fatalf("after seed: got %d, want 1", got)
	}

	// Empty set un-syncs the admin.
	if _, err := SetAdminMutelist(pkA, nil); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got := len(AdminMutelistPubkeys()); got != 0 {
		t.Errorf("after clear: got %d, want 0", got)
	}
	if got := len(GetAdminMutelistMeta()); got != 0 {
		t.Errorf("meta after clear: got %d entries, want 0", got)
	}
}

func TestAdminMutelistPubkeys_UnionAcrossAdmins(t *testing.T) {
	withDataDir(t)

	// Two admins, overlapping on pkB — union should dedupe to 3.
	if _, err := SetAdminMutelist(pkA, []string{pkB, pkC}); err != nil {
		t.Fatal(err)
	}
	if _, err := SetAdminMutelist(pkB, []string{pkB, pkA}); err != nil {
		t.Fatal(err)
	}
	if got := len(AdminMutelistPubkeys()); got != 3 {
		t.Errorf("union: got %d, want 3: %v", got, AdminMutelistPubkeys())
	}
}
