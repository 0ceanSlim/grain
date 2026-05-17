package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/0ceanslim/grain/tests"
)

// Phase 2 grain_* method tests.
//
// Scope: prove the dispatcher wiring works for each method —
// dispatcher accepts the params, calls the right config helper,
// returns the right response shape. Field-level state retention
// across round-trips is NOT validated here: the REST read
// endpoints expose only a subset of each config struct (e.g.
// LoggingConfigResponse omits Stdout), so a fetch-then-update
// flow loses fields by design. Production dashboards must send
// complete blobs; that's a dashboard concern, not a dispatcher
// concern.
//
// Tests therefore send minimal but valid blobs, assert the staged
// response shape, and rely on later list-read assertions only
// where the read returns the full struct (grain_whitelistconfig /
// grain_blacklistconfig). grain_reloadconfig is intentionally not
// exercised here — actually firing a restart inside the test would
// take down the relay between assertions.

func assertStaged(t *testing.T, env *nip86Reply) {
	t.Helper()
	if env == nil {
		t.Fatalf("nil envelope")
	}
	if env.Error != "" {
		t.Fatalf("unexpected error: %q", env.Error)
	}
	var r map[string]any
	if err := json.Unmarshal(env.Result, &r); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if ok, _ := r["ok"].(bool); !ok {
		t.Fatalf("result missing ok:true: %v", r)
	}
	if rp, _ := r["restart_pending"].(bool); !rp {
		t.Fatalf("result missing restart_pending:true: %v", r)
	}
}

// ─── update methods (wiring only — minimal blob each) ───────────

func TestNIP86_GrainUpdateRateLimit(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	payload := map[string]any{
		"ws_limit": 500.0, "ws_burst": 1000.0,
		"event_limit": 500.0, "event_burst": 1000.0,
		"req_limit": 500.0, "req_burst": 1000.0,
		"max_event_size":   524288.0,
		"kind_size_limits": []any{},
		"category_limits":  map[string]any{},
		"kind_limits":      []any{},
	}
	_, env := callNIP86(t, owner, "grain_updateratelimit", []any{payload})
	assertStaged(t, env)
}

func TestNIP86_GrainUpdateLogging(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	payload := map[string]any{
		"level":               "warn",
		"file":                "/app/testdata/debug",
		"max_log_size_mb":     10.0,
		"structure":           false,
		"stdout":              true,
		"check_interval_min":  10.0,
		"backup_count":        2.0,
		"suppress_components": []any{},
	}
	_, env := callNIP86(t, owner, "grain_updatelogging", []any{payload})
	assertStaged(t, env)
}

func TestNIP86_GrainUpdateAuth(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	payload := map[string]any{
		"required":        false,
		"relay_url":       "http://127.0.0.1:8190",
		"relay_url_match": "host",
	}
	_, env := callNIP86(t, owner, "grain_updateauth", []any{payload})
	assertStaged(t, env)
}

func TestNIP86_GrainUpdateEventPurge(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	payload := map[string]any{
		"enabled":                false,
		"disable_at_startup":     true,
		"keep_interval_hours":    24.0,
		"purge_interval_minutes": 240.0,
		"purge_by_category":      map[string]any{},
		"purge_by_kind_enabled":  false,
		"kinds_to_purge":         []any{},
		"exclude_whitelisted":    true,
	}
	_, env := callNIP86(t, owner, "grain_updateeventpurge", []any{payload})
	assertStaged(t, env)
}

func TestNIP86_GrainUpdateBackupRelay(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	payload := map[string]any{"enabled": false, "url": ""}
	_, env := callNIP86(t, owner, "grain_updatebackuprelay", []any{payload})
	assertStaged(t, env)
}

func TestNIP86_GrainUpdateResourceLimits(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	payload := map[string]any{
		"cpu_cores": 2.0, "memory_mb": 512.0, "heap_size_mb": 256.0,
	}
	_, env := callNIP86(t, owner, "grain_updateresourcelimits", []any{payload})
	assertStaged(t, env)
}

func TestNIP86_GrainUpdateEventTimeConstraints(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	payload := map[string]any{
		"min_created_at": 1577836800.0,
	}
	_, env := callNIP86(t, owner, "grain_updateeventtimeconstraints", []any{payload})
	assertStaged(t, env)
}

func TestNIP86_GrainUpdateServer(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	payload := map[string]any{
		"port":                         ":8190",
		"read_timeout":                 60.0,
		"write_timeout":                20.0,
		"idle_timeout":                 1200.0,
		"max_connections":              1000.0,
		"max_subscriptions_per_client": 25.0,
		"implicit_req_limit":           500.0,
		"connection_rate_limit_per_ip": 0.0,
	}
	_, env := callNIP86(t, owner, "grain_updateserver", []any{payload})
	assertStaged(t, env)
}

// Full-config read round-trip works for whitelist/blacklist because
// grain_<section>config returns the complete struct. These tests
// fetch via the new full-read methods, mutate one field, send back
// — and the cleanup restores the original.
func TestNIP86_GrainUpdateWhitelistConfig(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)

	_, env := callNIP86(t, owner, "grain_whitelistconfig", nil)
	if env == nil || env.Error != "" {
		t.Fatalf("read failed: %+v", env)
	}
	var current map[string]any
	if err := json.Unmarshal(env.Result, &current); err != nil {
		t.Fatalf("decode: %v", err)
	}
	t.Cleanup(func() {
		callNIP86(t, owner, "grain_updatewhitelistconfig", []any{current})
	})

	// Toggle pubkey_whitelist.enabled.
	pw, _ := current["pubkey_whitelist"].(map[string]any)
	origEnabled, _ := pw["enabled"].(bool)
	pw["enabled"] = !origEnabled

	_, env = callNIP86(t, owner, "grain_updatewhitelistconfig", []any{current})
	assertStaged(t, env)

	_, env = callNIP86(t, owner, "grain_whitelistconfig", nil)
	var after map[string]any
	_ = json.Unmarshal(env.Result, &after)
	apw, _ := after["pubkey_whitelist"].(map[string]any)
	if got, _ := apw["enabled"].(bool); got != !origEnabled {
		t.Fatalf("pubkey_whitelist.enabled not flipped: got %v want %v", got, !origEnabled)
	}
}

func TestNIP86_GrainUpdateBlacklistConfig(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)

	_, env := callNIP86(t, owner, "grain_blacklistconfig", nil)
	if env == nil || env.Error != "" {
		t.Fatalf("read failed: %+v", env)
	}
	var current map[string]any
	if err := json.Unmarshal(env.Result, &current); err != nil {
		t.Fatalf("decode: %v", err)
	}
	t.Cleanup(func() {
		callNIP86(t, owner, "grain_updateblacklistconfig", []any{current})
	})

	origEnabled, _ := current["enabled"].(bool)
	current["enabled"] = !origEnabled

	_, env = callNIP86(t, owner, "grain_updateblacklistconfig", []any{current})
	assertStaged(t, env)

	_, env = callNIP86(t, owner, "grain_blacklistconfig", nil)
	var after map[string]any
	_ = json.Unmarshal(env.Result, &after)
	if got, _ := after["enabled"].(bool); got != !origEnabled {
		t.Fatalf("blacklist.enabled not flipped: got %v want %v", got, !origEnabled)
	}
}

// ─── operational ─────────────────────────────────────────────────

func TestNIP86_GrainRefreshCache(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	_, env := callNIP86(t, owner, "grain_refreshcache", nil)
	if env == nil || env.Error != "" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	var stats map[string]any
	if err := json.Unmarshal(env.Result, &stats); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if _, ok := stats["whitelist_count"]; !ok {
		t.Fatalf("refreshcache result missing whitelist_count: %v", stats)
	}
}

// ─── reads ───────────────────────────────────────────────────────

func TestNIP86_GrainWhitelistConfig(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	_, env := callNIP86(t, owner, "grain_whitelistconfig", nil)
	if env == nil || env.Error != "" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	var wl map[string]any
	if err := json.Unmarshal(env.Result, &wl); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := wl["pubkey_whitelist"]; !ok {
		t.Fatalf("missing pubkey_whitelist section: %v", wl)
	}
}

func TestNIP86_GrainBlacklistConfig(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	_, env := callNIP86(t, owner, "grain_blacklistconfig", nil)
	if env == nil || env.Error != "" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	var bl map[string]any
	if err := json.Unmarshal(env.Result, &bl); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := bl["enabled"]; !ok {
		t.Fatalf("missing enabled field: %v", bl)
	}
}

// ─── stats ───────────────────────────────────────────────────────

func TestNIP86_GrainStatsOverview(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	_, env := callNIP86(t, owner, "grain_stats_overview", nil)
	if env == nil || env.Error != "" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	var stats map[string]any
	if err := json.Unmarshal(env.Result, &stats); err != nil {
		t.Fatalf("decode: %v", err)
	}
	server, ok := stats["server"].(map[string]any)
	if !ok {
		t.Fatalf("missing server section: %+v", stats)
	}
	if v, ok := server["active_connections"].(float64); !ok || v < 0 {
		t.Fatalf("invalid active_connections: %v", server["active_connections"])
	}
	if v, _ := server["uptime_seconds"].(float64); v <= 0 {
		t.Fatalf("expected uptime_seconds > 0, got %v", server["uptime_seconds"])
	}
	if _, ok := stats["whitelist"]; !ok {
		t.Fatalf("missing whitelist section")
	}
	if _, ok := stats["blacklist"]; !ok {
		t.Fatalf("missing blacklist section")
	}
}

func TestNIP86_SupportedMethodsIncludesGrainExtensions(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	_, env := callNIP86(t, owner, "supportedmethods", nil)
	if env == nil || env.Error != "" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	var methods []string
	if err := json.Unmarshal(env.Result, &methods); err != nil {
		t.Fatalf("decode: %v", err)
	}
	required := []string{
		"grain_updateserver",
		"grain_updateratelimit",
		"grain_updateeventpurge",
		"grain_updatelogging",
		"grain_updateauth",
		"grain_updatebackuprelay",
		"grain_updateresourcelimits",
		"grain_updateeventtimeconstraints",
		"grain_updatewhitelistconfig",
		"grain_updateblacklistconfig",
		"grain_reloadconfig",
		"grain_refreshcache",
		"grain_whitelistconfig",
		"grain_blacklistconfig",
		"grain_stats_overview",
	}
	for _, want := range required {
		found := false
		for _, m := range methods {
			if m == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("supportedmethods missing %q", want)
		}
	}
}

var _ = http.StatusOK // keep net/http import alive
