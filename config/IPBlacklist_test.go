package config

import (
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	cfgType "github.com/0ceanslim/grain/config/types"
)

func setupIPBlocklistTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prevDir := GetDataDir()
	SetDataDir(dir)
	t.Cleanup(func() {
		SetDataDir(prevDir)
		ResetIPBlocklistForTest()
	})
	ResetIPBlocklistForTest()
	return dir
}

func TestParseIPOrCIDR_AcceptsBoth(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"1.2.3.4", "1.2.3.4/32"},
		{"10.0.0.0/8", "10.0.0.0/8"},
		{"192.168.1.42/24", "192.168.1.0/24"}, // Masked normalises
		{"::1", "::1/128"},
		{"2001:db8::/32", "2001:db8::/32"},
	}
	for _, tc := range cases {
		got, err := parseIPOrCIDR(tc.in)
		if err != nil {
			t.Errorf("parseIPOrCIDR(%q) err: %v", tc.in, err)
			continue
		}
		if got.String() != tc.want {
			t.Errorf("parseIPOrCIDR(%q) = %q, want %q", tc.in, got.String(), tc.want)
		}
	}
}

func TestParseIPOrCIDR_RejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "not an ip", "999.999.999.999", "1.2.3.4/64", "1.2.3.0/abc"} {
		if _, err := parseIPOrCIDR(in); err == nil {
			t.Errorf("parseIPOrCIDR(%q) should have errored", in)
		}
	}
}

func TestParsePermanentIPPrefixes_SkipsInvalid(t *testing.T) {
	in := []string{"1.2.3.4", "garbage", "10.0.0.0/8", ""}
	out := ParsePermanentIPPrefixes(in)
	if len(out) != 2 {
		t.Fatalf("got %d prefixes, want 2 (garbage and empty should be skipped)", len(out))
	}
}

func TestIsIPBlocked_PermanentCIDRMatch(t *testing.T) {
	setupIPBlocklistTest(t)
	cfg := cfgType.BlacklistConfig{
		PermanentBlockedIPs: []string{"203.0.113.0/24", "198.51.100.42"},
	}
	LoadIPBlocklist(cfg)

	cases := []struct {
		ip    string
		block bool
	}{
		{"203.0.113.5", true},    // inside /24
		{"203.0.113.255", true},  // inside /24
		{"203.0.114.1", false},   // outside /24
		{"198.51.100.42", true},  // exact /32
		{"198.51.100.43", false}, // adjacent /32 — not blocked
		{"127.0.0.1", false},     // unrelated
	}
	for _, tc := range cases {
		blocked, _ := IsIPBlocked(tc.ip)
		if blocked != tc.block {
			t.Errorf("IsIPBlocked(%q) = %v, want %v", tc.ip, blocked, tc.block)
		}
	}
}

func TestIsIPBlocked_UnparseableIPNotBlocked(t *testing.T) {
	setupIPBlocklistTest(t)
	LoadIPBlocklist(cfgType.BlacklistConfig{PermanentBlockedIPs: []string{"1.2.3.4"}})
	for _, in := range []string{"", "not an ip", "999.999.999.999"} {
		if blocked, _ := IsIPBlocked(in); blocked {
			t.Errorf("IsIPBlocked(%q) returned blocked=true for unparseable input", in)
		}
	}
}

func TestRecordIPRateViolation_NoOpUnderThreshold(t *testing.T) {
	setupIPBlocklistTest(t)
	cfg := cfgType.BlacklistConfig{
		IPRateViolationThreshold: 5,
		IPMaxTempBans:            3,
		IPTempBanDuration:        60,
	}
	LoadIPBlocklist(cfg)

	for i := 0; i < 5; i++ {
		RecordIPRateViolation("1.2.3.4", cfg)
	}
	if blocked, _ := IsIPBlocked("1.2.3.4"); blocked {
		t.Fatal("IP should not be blocked at exactly threshold count")
	}
}

func TestRecordIPRateViolation_TempBanAfterThreshold(t *testing.T) {
	setupIPBlocklistTest(t)
	cfg := cfgType.BlacklistConfig{
		IPRateViolationThreshold: 3,
		IPMaxTempBans:            5,
		IPTempBanDuration:        60,
	}
	LoadIPBlocklist(cfg)

	// 3 violations: under threshold (>3 needed). 4th crosses.
	for i := 0; i < 4; i++ {
		RecordIPRateViolation("2.2.2.2", cfg)
	}
	blocked, reason := IsIPBlocked("2.2.2.2")
	if !blocked {
		t.Fatal("IP should be temp-banned after 4 violations with threshold=3")
	}
	if reason != "temp" {
		t.Errorf("reason = %q, want %q", reason, "temp")
	}
}

func TestRecordIPRateViolation_PromotesToPermanentAndPersists(t *testing.T) {
	dir := setupIPBlocklistTest(t)
	cfg := cfgType.BlacklistConfig{
		IPRateViolationThreshold: 1,
		IPMaxTempBans:            2,
		IPTempBanDuration:        60,
	}
	LoadIPBlocklist(cfg)

	// Each pair of violations → 1 temp ban (threshold=1 means >1 trips).
	// We need (max+1) = 3 temp bans to promote, so 6 violations total.
	for i := 0; i < 6; i++ {
		RecordIPRateViolation("9.9.9.9", cfg)
	}

	blocked, reason := IsIPBlocked("9.9.9.9")
	if !blocked {
		t.Fatal("IP should be permanently banned after exceeding max temp bans")
	}
	if reason != "permanent" {
		t.Errorf("reason = %q, want %q", reason, "permanent")
	}

	// Sidecar must exist on disk and contain the entry.
	path := filepath.Join(dir, ipSidecarFile)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("sidecar not written: %v", err)
	}
	var s ipSidecar
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("sidecar invalid JSON: %v", err)
	}
	if s.Version != ipSidecarVersion {
		t.Errorf("version = %d, want %d", s.Version, ipSidecarVersion)
	}
	if len(s.Permanent) != 1 || s.Permanent[0].Prefix != "9.9.9.9/32" {
		t.Errorf("permanent entries = %+v, want 1 entry for 9.9.9.9/32", s.Permanent)
	}
	if s.Permanent[0].Reason == "" {
		t.Error("entry missing reason")
	}
	if s.Permanent[0].AddedAt == 0 {
		t.Error("entry missing added_at")
	}
}

func TestLoadIPBlocklist_LoadsSidecar(t *testing.T) {
	dir := setupIPBlocklistTest(t)
	// Write a fixture sidecar with one auto-banned /32.
	fixture := ipSidecar{
		Version: ipSidecarVersion,
		Permanent: []ipPermanentEntry{
			{Prefix: "172.16.0.0/16", AddedAt: time.Now().Unix(), Reason: "manual fixture"},
		},
	}
	data, _ := json.Marshal(fixture)
	if err := os.WriteFile(filepath.Join(dir, ipSidecarFile), data, 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	LoadIPBlocklist(cfgType.BlacklistConfig{
		PermanentBlockedIPs: []string{"10.0.0.0/8"}, // admin entry merged
	})

	// Both admin and sidecar entries should match.
	for _, ip := range []string{"172.16.42.1", "10.5.5.5"} {
		blocked, _ := IsIPBlocked(ip)
		if !blocked {
			t.Errorf("IsIPBlocked(%q) = false, expected blocked from %s source", ip,
				map[string]string{"172.16.42.1": "sidecar", "10.5.5.5": "admin"}[ip])
		}
	}
}

func TestLoadIPBlocklist_MalformedSidecarStartsEmpty(t *testing.T) {
	dir := setupIPBlocklistTest(t)
	if err := os.WriteFile(filepath.Join(dir, ipSidecarFile), []byte("not json"), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	LoadIPBlocklist(cfgType.BlacklistConfig{}) // should not panic
	if blocked, _ := IsIPBlocked("1.2.3.4"); blocked {
		t.Error("IP unexpectedly blocked after malformed sidecar load")
	}
}

func TestSweepExpiredIPTempBans_RemovesExpired(t *testing.T) {
	setupIPBlocklistTest(t)
	cfg := cfgType.BlacklistConfig{
		IPRateViolationThreshold: 0, // disable escalation, drive the map directly
	}
	LoadIPBlocklist(cfg)

	// Manually inject one expired and one active temp ban.
	ipMu.Lock()
	ipTempBans["expired"] = &ipTempBanEntry{count: 1, unbanTime: time.Now().Add(-time.Hour)}
	ipTempBans["active"] = &ipTempBanEntry{count: 1, unbanTime: time.Now().Add(time.Hour)}
	ipMu.Unlock()

	SweepExpiredIPTempBans()

	ipMu.Lock()
	defer ipMu.Unlock()
	if _, ok := ipTempBans["expired"]; ok {
		t.Error("expired entry was not swept")
	}
	if _, ok := ipTempBans["active"]; !ok {
		t.Error("active entry was incorrectly swept")
	}
}

func TestRecordIPRateViolation_DisabledWhenAllZero(t *testing.T) {
	setupIPBlocklistTest(t)
	cfg := cfgType.BlacklistConfig{} // all zero
	LoadIPBlocklist(cfg)
	for i := 0; i < 100; i++ {
		RecordIPRateViolation("3.3.3.3", cfg)
	}
	if blocked, _ := IsIPBlocked("3.3.3.3"); blocked {
		t.Fatal("IP should not be blocked when all escalation thresholds are zero")
	}
}

func TestIsIPBlocked_TempBanReturnsTempReason(t *testing.T) {
	setupIPBlocklistTest(t)
	LoadIPBlocklist(cfgType.BlacklistConfig{})
	ipMu.Lock()
	ipTempBans["4.4.4.4"] = &ipTempBanEntry{count: 1, unbanTime: time.Now().Add(time.Hour)}
	ipMu.Unlock()
	blocked, reason := IsIPBlocked("4.4.4.4")
	if !blocked || reason != "temp" {
		t.Errorf("got blocked=%v reason=%q, want true/temp", blocked, reason)
	}
}

// IPv6 sanity: a /64 admin entry covers any /128 inside.
func TestIsIPBlocked_IPv6CIDR(t *testing.T) {
	setupIPBlocklistTest(t)
	LoadIPBlocklist(cfgType.BlacklistConfig{
		PermanentBlockedIPs: []string{"2001:db8::/32"},
	})
	if blocked, _ := IsIPBlocked("2001:db8:1234::1"); !blocked {
		t.Error("inside-prefix v6 not blocked")
	}
	if blocked, _ := IsIPBlocked("2001:db9::1"); blocked {
		t.Error("outside-prefix v6 incorrectly blocked")
	}
	// Smoke check: prefix list parsed at least one entry.
	if len(permanentPrefixes) == 0 {
		t.Error("expected v6 prefix to be loaded")
	}
	_ = netip.MustParseAddr // import retained for table-driven helpers if expanded later
}
