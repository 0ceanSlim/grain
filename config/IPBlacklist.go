package config

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// IP-level blacklist with CIDR support and auto-escalation. Mirrors the
// pubkey blacklist's temp→permanent escalation pattern at the network
// layer. The HTTP middleware that runs before the WebSocket upgrade
// (see #61) checks IsIPBlocked for each incoming connection; the
// connection rate limiter calls RecordIPRateViolation on every
// over-limit hit so this state machine sees rate-limit pressure
// directly.
//
// Permanent bans live in two sources merged into one in-memory list:
//   - Admin-curated, in blacklist.yml under permanent_blocked_ips.
//   - Auto-escalated, in <data_dir>/ip_bans.json. The admin source is
//     human-edited; the sidecar is machine-managed. Keeping them
//     separate means an admin reading the YAML doesn't see auto-bans
//     mixed in.
//
// Temp bans are in-memory only — same as the pubkey path. A restart
// clears them; permanent bans persist via the sidecar.
//
// All public functions are safe for concurrent use.

// ipBanReason values are written to the sidecar JSON for forensics.
const (
	ipBanReasonRateEscalation = "rate-limit escalation"
)

// ipSidecarVersion is the on-disk format version, in case the schema
// has to evolve later.
const ipSidecarVersion = 1

const ipSidecarFile = "ip_bans.json"

// ipPermanentEntry is the JSON-serialised form of a single permanent
// auto-ban. The admin-curated entries from config don't end up here —
// they live in blacklist.yml.
type ipPermanentEntry struct {
	Prefix  string `json:"prefix"`
	AddedAt int64  `json:"added_at"`
	Reason  string `json:"reason"`
}

type ipSidecar struct {
	Version   int                `json:"version"`
	Permanent []ipPermanentEntry `json:"permanent"`
}

type ipTempBanEntry struct {
	count     int
	unbanTime time.Time
}

var (
	ipMu sync.Mutex

	// permanentPrefixes is the merged list of admin- and sidecar-derived
	// permanent CIDRs. Order is irrelevant; lookup walks all of them.
	permanentPrefixes []netip.Prefix

	// sidecarPermanent is the slice of auto-escalated permanent entries
	// loaded from / persisted to <data_dir>/ip_bans.json. Kept separate
	// from permanentPrefixes so we can write the sidecar back without
	// re-serialising admin-curated entries.
	sidecarPermanent []ipPermanentEntry

	// ipViolations counts rate-limit hits per IP toward the next temp ban.
	// Cleared when a temp ban is issued.
	ipViolations = make(map[string]int)

	// ipTempBans tracks active and historical temp bans per IP. The
	// count keeps growing across re-bans so it can drive the
	// promotion-to-permanent threshold.
	ipTempBans = make(map[string]*ipTempBanEntry)
)

// ParsePermanentIPPrefixes converts a slice of strings (CIDRs or bare IPs)
// into netip.Prefix values. Invalid entries are skipped with a WARN log.
// A bare IP becomes a /32 (IPv4) or /128 (IPv6) prefix.
func ParsePermanentIPPrefixes(entries []string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(entries))
	for _, e := range entries {
		p, err := parseIPOrCIDR(e)
		if err != nil {
			log.Config().Warn("Invalid IP blacklist entry, skipping", "entry", e, "error", err)
			continue
		}
		prefixes = append(prefixes, p)
	}
	return prefixes
}

// parseIPOrCIDR accepts either "1.2.3.4" or "1.2.3.0/24" and returns the
// canonical Prefix. Bare addresses become /32 or /128.
func parseIPOrCIDR(s string) (netip.Prefix, error) {
	if p, err := netip.ParsePrefix(s); err == nil {
		return p.Masked(), nil
	}
	if a, err := netip.ParseAddr(s); err == nil {
		bits := 32
		if a.Is6() && !a.Is4In6() {
			bits = 128
		}
		return netip.PrefixFrom(a, bits), nil
	}
	return netip.Prefix{}, fmt.Errorf("not a valid IP or CIDR: %q", s)
}

// LoadIPBlocklist initialises the in-memory permanent prefix list from the
// admin-curated config and the on-disk sidecar. Safe to call multiple
// times — subsequent calls replace the in-memory state. Should be called
// once at startup, after SetDataDir.
func LoadIPBlocklist(cfg cfgType.BlacklistConfig) {
	adminPrefixes := ParsePermanentIPPrefixes(cfg.PermanentBlockedIPs)

	sidecarEntries, err := loadIPSidecar()
	if err != nil {
		log.Config().Warn("Failed to load IP ban sidecar, starting empty",
			"path", ipSidecarPath(), "error", err)
		sidecarEntries = nil
	}
	sidecarPrefixes := make([]netip.Prefix, 0, len(sidecarEntries))
	for _, e := range sidecarEntries {
		p, err := parseIPOrCIDR(e.Prefix)
		if err != nil {
			log.Config().Warn("Invalid sidecar IP entry, skipping", "prefix", e.Prefix, "error", err)
			continue
		}
		sidecarPrefixes = append(sidecarPrefixes, p)
	}

	ipMu.Lock()
	permanentPrefixes = append(adminPrefixes, sidecarPrefixes...)
	sidecarPermanent = sidecarEntries
	ipViolations = make(map[string]int)
	ipTempBans = make(map[string]*ipTempBanEntry)
	count := len(permanentPrefixes)
	ipMu.Unlock()

	log.Config().Info("IP blocklist loaded",
		"admin_entries", len(adminPrefixes),
		"sidecar_entries", len(sidecarPrefixes),
		"total_permanent", count)
}

// IsIPBlocked returns (true, reason) if the given IP string matches any
// permanent CIDR or has an active temp ban. The reason is suitable for
// log attribution.
func IsIPBlocked(ip string) (bool, string) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		// Unparseable IPs (empty string, malformed XFF) are not blocked
		// here — the upstream layer handled them already.
		return false, ""
	}
	ipMu.Lock()
	defer ipMu.Unlock()

	for _, p := range permanentPrefixes {
		if p.Contains(addr) {
			return true, "permanent"
		}
	}
	if entry, ok := ipTempBans[ip]; ok {
		if time.Now().Before(entry.unbanTime) {
			return true, "temp"
		}
	}
	return false, ""
}

// RecordIPRateViolation is the auto-escalation hook. The connection rate
// limiter calls this on every per-IP rate-limit rejection. Behaviour
// depends on cfg.IPRateViolationThreshold and IPMaxTempBans: when
// violations cross the threshold a temp ban is issued; once temp bans
// for that IP exceed the max, the IP is promoted to permanent and
// persisted to the sidecar.
//
// All thresholds <= 0 disable the corresponding stage. With everything
// at 0 this function is a no-op and the IP escalation pipeline is off.
func RecordIPRateViolation(ip string, cfg cfgType.BlacklistConfig) {
	if ip == "" {
		return
	}
	if cfg.IPRateViolationThreshold <= 0 && cfg.IPMaxTempBans <= 0 {
		return
	}
	ipMu.Lock()

	ipViolations[ip]++
	violations := ipViolations[ip]

	if cfg.IPRateViolationThreshold <= 0 || violations <= cfg.IPRateViolationThreshold {
		ipMu.Unlock()
		return
	}

	// Threshold crossed — issue a temp ban and reset the violation counter.
	delete(ipViolations, ip)

	entry, ok := ipTempBans[ip]
	if !ok {
		entry = &ipTempBanEntry{}
		ipTempBans[ip] = entry
	}
	entry.count++
	if cfg.IPTempBanDuration > 0 {
		entry.unbanTime = time.Now().Add(time.Duration(cfg.IPTempBanDuration) * time.Second)
	}
	tempCount := entry.count

	log.Config().Warn("IP temp-banned for rate-limit escalation",
		"ip", ip,
		"temp_ban_count", tempCount,
		"max_temp_bans", cfg.IPMaxTempBans,
		"unban_time", entry.unbanTime.Format(time.RFC3339))

	if cfg.IPMaxTempBans <= 0 || tempCount <= cfg.IPMaxTempBans {
		ipMu.Unlock()
		return
	}

	// Promotion to permanent. Drop the temp entry, append a /32 (or /128)
	// to permanent, persist sidecar.
	delete(ipTempBans, ip)
	prefix, perr := parseIPOrCIDR(ip)
	if perr != nil {
		log.Config().Error("Cannot promote IP to permanent: parse failed", "ip", ip, "error", perr)
		ipMu.Unlock()
		return
	}
	permanentPrefixes = append(permanentPrefixes, prefix)
	sidecarPermanent = append(sidecarPermanent, ipPermanentEntry{
		Prefix:  prefix.String(),
		AddedAt: time.Now().Unix(),
		Reason:  ipBanReasonRateEscalation,
	})
	snapshot := make([]ipPermanentEntry, len(sidecarPermanent))
	copy(snapshot, sidecarPermanent)
	ipMu.Unlock()

	log.Config().Warn("IP promoted to permanent blacklist", "ip", ip, "prefix", prefix.String())

	if err := writeIPSidecar(snapshot); err != nil {
		log.Config().Error("Failed to persist IP ban sidecar", "error", err)
	}
}

// SweepExpiredIPTempBans removes temp ban entries past their unbanTime.
// Caller is expected to schedule this periodically; see
// StartIPBlocklistSweeper for the canonical loop.
func SweepExpiredIPTempBans() {
	ipMu.Lock()
	defer ipMu.Unlock()
	now := time.Now()
	for ip, entry := range ipTempBans {
		if !entry.unbanTime.IsZero() && now.After(entry.unbanTime) {
			delete(ipTempBans, ip)
		}
	}
}

// StartIPBlocklistSweeper kicks off the background goroutine that
// expires temp bans every minute. Idempotent at the call site only —
// don't call this more than once.
func StartIPBlocklistSweeper() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			SweepExpiredIPTempBans()
		}
	}()
}

// ipSidecarPath returns the absolute path to ip_bans.json under the
// configured data dir. Empty string if the data dir hasn't been set yet.
func ipSidecarPath() string {
	dir := GetDataDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, ipSidecarFile)
}

func loadIPSidecar() ([]ipPermanentEntry, error) {
	path := ipSidecarPath()
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s ipSidecar
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return s.Permanent, nil
}

func writeIPSidecar(entries []ipPermanentEntry) error {
	path := ipSidecarPath()
	if path == "" {
		return fmt.Errorf("data dir not set")
	}
	s := ipSidecar{
		Version:   ipSidecarVersion,
		Permanent: entries,
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode sidecar: %w", err)
	}
	// Write atomically via tmp+rename so a crash mid-write doesn't
	// truncate the file. os.Rename is atomic on POSIX and atomic-enough
	// on Windows for our purposes.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename tmp: %w", err)
	}
	return nil
}

// ResetIPBlocklistForTest clears all in-memory IP-blocklist state. Tests
// only — production code paths use LoadIPBlocklist.
func ResetIPBlocklistForTest() {
	ipMu.Lock()
	defer ipMu.Unlock()
	permanentPrefixes = nil
	sidecarPermanent = nil
	ipViolations = make(map[string]int)
	ipTempBans = make(map[string]*ipTempBanEntry)
}
