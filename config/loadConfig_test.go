package config

import (
	"testing"

	cfgType "github.com/0ceanslim/grain/config/types"
)

// normalizeLegacyConfig is what lets an operator drop in a pre-existing
// config without hand-editing YAML: deprecated shapes are corrected in
// memory on load, and the admin form/dashboard then persist the fix on
// the next save. These tests pin that behavior.

func TestNormalizeLegacyConfig_BackupRelayURLMigration(t *testing.T) {
	// Old single-URL config: url set, urls absent -> folded into urls.
	c := &cfgType.ServerConfig{
		BackupRelay: cfgType.BackupRelayConfig{URL: "wss://old.example"},
	}
	normalizeLegacyConfig(c)
	if c.BackupRelay.URL != "" {
		t.Errorf("deprecated URL should be cleared, got %q", c.BackupRelay.URL)
	}
	if len(c.BackupRelay.URLs) != 1 || c.BackupRelay.URLs[0] != "wss://old.example" {
		t.Errorf("URL should migrate into URLs, got %v", c.BackupRelay.URLs)
	}
}

func TestNormalizeLegacyConfig_BackupRelayNoClobber(t *testing.T) {
	// Both set (mixed old+new hand edit): keep the urls list, just drop
	// the deprecated scalar — never silently discard configured urls.
	c := &cfgType.ServerConfig{
		BackupRelay: cfgType.BackupRelayConfig{
			URL:  "wss://old.example",
			URLs: []string{"wss://new.example"},
		},
	}
	normalizeLegacyConfig(c)
	if c.BackupRelay.URL != "" {
		t.Errorf("deprecated URL should be cleared, got %q", c.BackupRelay.URL)
	}
	if len(c.BackupRelay.URLs) != 1 || c.BackupRelay.URLs[0] != "wss://new.example" {
		t.Errorf("existing URLs must not be clobbered, got %v", c.BackupRelay.URLs)
	}
}

func TestNormalizeLegacyConfig_BackupRelayModernNoop(t *testing.T) {
	c := &cfgType.ServerConfig{
		BackupRelay: cfgType.BackupRelayConfig{URLs: []string{"wss://a", "wss://b"}},
	}
	normalizeLegacyConfig(c)
	if c.BackupRelay.URL != "" || len(c.BackupRelay.URLs) != 2 {
		t.Errorf("modern config should be untouched, got url=%q urls=%v",
			c.BackupRelay.URL, c.BackupRelay.URLs)
	}
}

func TestNormalizeLegacyConfig_PurgeCategoryAlias(t *testing.T) {
	// Old category key "parameterized_replaceable" -> canonical
	// "addressable" so the admin form (which keys on "addressable")
	// mirrors the stored value instead of rendering it unchecked.
	c := &cfgType.ServerConfig{
		EventPurge: cfgType.EventPurgeConfig{
			PurgeByCategory: map[string]bool{
				"regular":                   true,
				"parameterized_replaceable": true,
			},
		},
	}
	normalizeLegacyConfig(c)
	m := c.EventPurge.PurgeByCategory
	if _, stillOld := m["parameterized_replaceable"]; stillOld {
		t.Error("legacy category key should be removed")
	}
	if !m["addressable"] {
		t.Errorf("legacy value should move to addressable, got %v", m)
	}
	if !m["regular"] {
		t.Error("unrelated categories must be preserved")
	}
}

func TestNormalizeLegacyConfig_PurgeCategoryAliasNoClobber(t *testing.T) {
	// If a config somehow carries both keys, the canonical one wins and
	// the legacy one is dropped without overwriting it.
	c := &cfgType.ServerConfig{
		EventPurge: cfgType.EventPurgeConfig{
			PurgeByCategory: map[string]bool{
				"addressable":               true,
				"parameterized_replaceable": false,
			},
		},
	}
	normalizeLegacyConfig(c)
	m := c.EventPurge.PurgeByCategory
	if _, stillOld := m["parameterized_replaceable"]; stillOld {
		t.Error("legacy category key should be removed")
	}
	if !m["addressable"] {
		t.Error("existing addressable value must not be clobbered by the legacy key")
	}
}

func TestNormalizeLegacyConfig_NilPurgeMap(t *testing.T) {
	// A config with no purge categories at all must not panic.
	c := &cfgType.ServerConfig{}
	normalizeLegacyConfig(c)
	if c.EventPurge.PurgeByCategory != nil {
		t.Error("nil category map should stay nil")
	}
}
