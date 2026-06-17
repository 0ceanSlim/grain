package core

import "testing"

func hasTag(tags [][]string, key, val string) bool {
	for _, t := range tags {
		if len(t) >= 2 && t[0] == key && t[1] == val {
			return true
		}
	}
	return false
}

func countTag(tags [][]string, key string) int {
	n := 0
	for _, t := range tags {
		if len(t) >= 1 && t[0] == key {
			n++
		}
	}
	return n
}

func TestApplyClientTag(t *testing.T) {
	base := [][]string{{"d", "x"}, {"client", "amethyst"}, {"r", "wss://relay.example"}}

	// Enabled: foreign stripped, grain's own added exactly once, others kept.
	on := applyClientTag(base, true, "grain")
	if hasTag(on, "client", "amethyst") {
		t.Error("foreign client tag should be stripped")
	}
	if !hasTag(on, "client", "grain") {
		t.Error("grain client tag should be added")
	}
	if countTag(on, "client") != 1 {
		t.Errorf("expected exactly one client tag, got %d", countTag(on, "client"))
	}
	if !hasTag(on, "d", "x") || !hasTag(on, "r", "wss://relay.example") {
		t.Error("non-client tags must be preserved")
	}

	// Disabled: foreign stripped, none added.
	off := applyClientTag(base, false, "grain")
	if countTag(off, "client") != 0 {
		t.Errorf("disabled should leave no client tag, got %d", countTag(off, "client"))
	}

	// Empty name falls back to the default.
	def := applyClientTag(nil, true, "")
	if !hasTag(def, "client", DefaultClientTagName) {
		t.Errorf("empty name should default to %q", DefaultClientTagName)
	}

	// Input slice is not mutated.
	if !hasTag(base, "client", "amethyst") {
		t.Error("input slice must not be mutated")
	}
}
