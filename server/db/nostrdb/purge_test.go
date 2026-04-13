package nostrdb

import "testing"

// These tests verify the v0.4 purge_by_category compatibility rules.
// They're pure logic — no NDB open — so they run even when CGO linking
// of libnostrdb_full is unavailable on the host, and guard against
// category-name regressions that would silently break operator configs.

func TestPurgeCategoryForKind(t *testing.T) {
	cases := []struct {
		kind int
		want string
	}{
		{0, "replaceable"},
		{3, "replaceable"},
		{10002, "replaceable"},
		{1, "regular"},
		{4, "regular"},
		{44, "regular"},
		{1000, "regular"},
		{9999, "regular"},
		{2, "deprecated"},
		{20001, "ephemeral"},
		{30000, "parameterized_replaceable"},
		{39999, "parameterized_replaceable"},
		{45, "unknown"},
		{999, "unknown"},
		{40000, "unknown"},
	}
	for _, c := range cases {
		if got := purgeCategoryForKind(c.kind); got != c.want {
			t.Errorf("purgeCategoryForKind(%d) = %q, want %q", c.kind, got, c.want)
		}
	}
}

func TestCategoryPermitsPurge_V04Config(t *testing.T) {
	// v0.4 operator config — purge only regular + deprecated, keep
	// replaceable profiles and addressable lists.
	cfg := map[string]bool{
		"regular":                   true,
		"replaceable":               false,
		"parameterized_replaceable": false,
		"deprecated":                true,
	}
	cases := []struct {
		kind int
		want bool
	}{
		{1, true},      // regular -> purge
		{0, false},     // replaceable (kind-0 profile) -> keep
		{3, false},     // replaceable (follow list) -> keep
		{30000, false}, // addressable -> keep
		{2, true},      // deprecated -> purge
	}
	for _, c := range cases {
		if got := categoryPermitsPurge(c.kind, cfg); got != c.want {
			t.Errorf("categoryPermitsPurge(kind=%d) = %v, want %v", c.kind, got, c.want)
		}
	}
}

func TestCategoryPermitsPurge_V05AliasAccepted(t *testing.T) {
	// v0.5-style config using "addressable" name — legacy v0.4 code
	// categorized this as "parameterized_replaceable". The alias layer
	// should let a config using either name purge 30000-series kinds.
	cfg := map[string]bool{"addressable": true}
	if !categoryPermitsPurge(30000, cfg) {
		t.Errorf(`config {"addressable": true} should permit purging kind 30000`)
	}
	cfg = map[string]bool{"parameterized_replaceable": true}
	if !categoryPermitsPurge(30000, cfg) {
		t.Errorf(`config {"parameterized_replaceable": true} should permit purging kind 30000`)
	}
}

func TestCategoryPermitsPurge_MissingCategoryKeeps(t *testing.T) {
	// An empty/sparse category map must NOT implicitly allow purging
	// of unlisted categories — v0.4 semantics: only explicit `true`
	// entries permit deletion.
	cfg := map[string]bool{"regular": true}
	if categoryPermitsPurge(0, cfg) {
		t.Errorf("kind 0 (replaceable) should be kept when only regular:true is configured")
	}
	if categoryPermitsPurge(2, cfg) {
		t.Errorf("kind 2 (deprecated) should be kept when only regular:true is configured")
	}
}
