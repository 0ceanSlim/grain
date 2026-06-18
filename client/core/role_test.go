package core

import "testing"

func TestRoleHas(t *testing.T) {
	combo := RoleOutbox | RoleInbox
	if !combo.Has(RoleOutbox) {
		t.Error("combo should have RoleOutbox")
	}
	if !combo.Has(RoleInbox) {
		t.Error("combo should have RoleInbox")
	}
	if !combo.Has(RoleOutbox | RoleInbox) {
		t.Error("combo should have both bits")
	}
	if combo.Has(RoleDMInbox) {
		t.Error("combo should not have RoleDMInbox")
	}
	if combo.Has(RoleOutbox | RoleDMInbox) {
		t.Error("Has must require ALL bits, not any")
	}
	if (RoleOutbox).Has(0) {
		t.Error("Has(0) must be false")
	}
}

func TestRoleString(t *testing.T) {
	cases := map[Role]string{
		0:                        "none",
		RoleOutbox:               "outbox",
		RoleOutbox | RoleInbox:   "outbox|inbox",
		RoleDMInbox:              "dm-inbox",
		RoleTrusted:              "trusted",
		RoleSearch | RoleProxy:   "search|proxy",
		RoleOutbox | RoleTrusted: "outbox|trusted",
	}
	for r, want := range cases {
		if got := r.String(); got != want {
			t.Errorf("Role(%d).String() = %q, want %q", uint16(r), got, want)
		}
	}
	// String renders in bit order regardless of how the mask was composed.
	if got := (RoleInbox | RoleOutbox).String(); got != "outbox|inbox" {
		t.Errorf("bit-order render = %q, want %q", got, "outbox|inbox")
	}
}

func TestRoleForListKind(t *testing.T) {
	cases := []struct {
		kind int
		want Role
		ok   bool
	}{
		{10002, RoleOutbox | RoleInbox, true},
		{10050, RoleDMInbox, true},
		{10006, RoleBlocked, true},
		{10007, RoleSearch, true},
		{10012, RoleFavorite, true},
		{10013, RolePrivateHome, true},
		{0, 0, false},
		{1, 0, false},
		{30002, 0, false},
	}
	for _, c := range cases {
		got, ok := RoleForListKind(c.kind)
		if ok != c.ok || got != c.want {
			t.Errorf("RoleForListKind(%d) = (%v, %v), want (%v, %v)", c.kind, got, ok, c.want, c.ok)
		}
	}
}

func TestUserRelaysForRole(t *testing.T) {
	ur := &UserRelays{
		Outbox:  []string{"wss://out"},
		Inbox:   []string{"wss://in"},
		DMInbox: []string{"wss://dm"},
	}
	if got := ur.ForRole(RoleOutbox); len(got) != 1 || got[0] != "wss://out" {
		t.Errorf("ForRole(RoleOutbox) = %v", got)
	}
	if got := ur.ForRole(RoleInbox); len(got) != 1 || got[0] != "wss://in" {
		t.Errorf("ForRole(RoleInbox) = %v", got)
	}
	if got := ur.ForRole(RoleDMInbox); len(got) != 1 || got[0] != "wss://dm" {
		t.Errorf("ForRole(RoleDMInbox) = %v", got)
	}
	// Self-only / local roles aren't part of a per-target resolution.
	if got := ur.ForRole(RoleSearch); got != nil {
		t.Errorf("ForRole(RoleSearch) = %v, want nil", got)
	}
	// nil receiver is safe.
	var nilUR *UserRelays
	if got := nilUR.ForRole(RoleOutbox); got != nil {
		t.Errorf("nil.ForRole = %v, want nil", got)
	}
}
