package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/0ceanslim/grain/server/utils/log"
)

// Admin private mute list cache (#60).
//
// Most NIP-51 mute entries live in the event's NIP-44/NIP-04-encrypted
// `.content`, which the relay can't read — it doesn't hold the author's
// private key. The public-tag path (config/Blacklist.go) only ever sees
// `p`-tag entries. To close that gap for the relay owner's OWN mute list,
// decryption happens in the browser (where the owner's signer is wired up)
// and the resulting plain pubkey set is POSTed to the admin sync endpoint.
//
// This file is the persistence + merge layer for that pushed set: a sidecar
// JSON keyed by admin pubkey, loaded into the blacklist cache at startup and
// folded into every RefreshBlacklist as a fourth source (after direct
// pubkeys, npubs, and the public-mutelist fetch).
//
// Keyed by pubkey rather than a single owner slot so the shape survives a
// future move to a multi-admin gate without a migration; today the sync
// endpoint only ever writes the relay owner's key.

const (
	adminMutelistSidecarFile = "admin-mutelist-cache.json"
	adminMutelistVersion     = 1
)

// adminMutelistEntry is one admin's last synced private mute set.
type adminMutelistEntry struct {
	Pubkeys  []string `json:"pubkeys"`
	SyncedAt int64    `json:"synced_at"` // unix seconds
}

// adminMutelistSidecar is the on-disk shape of admin-mutelist-cache.json.
type adminMutelistSidecar struct {
	Version int                           `json:"version"`
	Admins  map[string]adminMutelistEntry `json:"admins"`
}

// AdminMutelistMeta is the per-admin summary the dashboard renders so the
// owner knows when they last synced and how many pubkeys they contributed.
type AdminMutelistMeta struct {
	Pubkey   string
	Count    int
	SyncedAt int64
}

var (
	adminMutelistMu    sync.RWMutex
	adminMutelistCache = make(map[string]adminMutelistEntry)
)

// LoadAdminMutelist reads the sidecar into memory. Safe to call before the
// DB is up; call once at startup after SetDataDir. A missing file is not an
// error — it just means no admin has synced yet.
func LoadAdminMutelist() {
	path := adminMutelistSidecarPath()
	if path == "" {
		log.Config().Warn("Admin mutelist sidecar load skipped: data dir not set")
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Config().Warn("Failed to read admin mutelist sidecar, starting empty",
			"path", path, "error", err)
		return
	}
	var s adminMutelistSidecar
	if err := json.Unmarshal(data, &s); err != nil {
		log.Config().Error("Failed to decode admin mutelist sidecar, starting empty",
			"path", path, "error", err)
		return
	}

	adminMutelistMu.Lock()
	adminMutelistCache = s.Admins
	if adminMutelistCache == nil {
		adminMutelistCache = make(map[string]adminMutelistEntry)
	}
	adminMutelistMu.Unlock()

	log.Config().Info("Loaded admin mutelist sidecar",
		"admins", len(s.Admins),
		"total_pubkeys", len(AdminMutelistPubkeys()))
}

// SetAdminMutelist records the decrypted pubkey set an admin pushed via the
// sync endpoint, persists it to the sidecar, and refreshes the blacklist
// cache so the new entries take effect immediately. Pubkeys are lowercased,
// trimmed, validated as 64-char hex, and deduplicated before storage —
// callers may pass raw client input. Passing an empty set clears that
// admin's contribution (a deliberate "un-sync").
func SetAdminMutelist(adminPubkey string, pubkeys []string) (AdminMutelistMeta, error) {
	clean := normalizePubkeySet(pubkeys)

	adminMutelistMu.Lock()
	if len(clean) == 0 {
		delete(adminMutelistCache, adminPubkey)
	} else {
		adminMutelistCache[adminPubkey] = adminMutelistEntry{
			Pubkeys:  clean,
			SyncedAt: time.Now().Unix(),
		}
	}
	snapshot := cloneAdminMutelist(adminMutelistCache)
	entry := adminMutelistCache[adminPubkey]
	adminMutelistMu.Unlock()

	if err := writeAdminMutelistSidecar(snapshot); err != nil {
		return AdminMutelistMeta{}, err
	}

	// Fold the new set into the validation cache now rather than waiting
	// for the next scheduled refresh — the owner just muted someone and
	// expects it to bite immediately.
	if cache := GetPubkeyCache(); cache != nil {
		if err := cache.RefreshBlacklist(); err != nil {
			log.Config().Warn("Failed to refresh blacklist after admin mutelist sync", "error", err)
		}
	}

	log.Config().Info("Admin mutelist synced",
		"admin", adminPubkey, "pubkeys", len(clean))
	return AdminMutelistMeta{
		Pubkey:   adminPubkey,
		Count:    len(entry.Pubkeys),
		SyncedAt: entry.SyncedAt,
	}, nil
}

// AdminMutelistPubkeys returns the deduplicated union of every admin's synced
// pubkeys. This is the fourth blacklist source consumed by RefreshBlacklist.
func AdminMutelistPubkeys() []string {
	adminMutelistMu.RLock()
	defer adminMutelistMu.RUnlock()

	seen := make(map[string]bool)
	var out []string
	for _, entry := range adminMutelistCache {
		for _, pk := range entry.Pubkeys {
			if !seen[pk] {
				seen[pk] = true
				out = append(out, pk)
			}
		}
	}
	return out
}

// GetAdminMutelistPubkeysFor returns a copy of one admin's stored pubkeys,
// sorted. Used by the owner-only admin panel to render the synced keys with
// profiles — safe there because the dashboard is gated to the relay owner,
// unlike the public keys endpoint which only ever sees a count.
func GetAdminMutelistPubkeysFor(adminPubkey string) []string {
	adminMutelistMu.RLock()
	defer adminMutelistMu.RUnlock()

	entry, ok := adminMutelistCache[adminPubkey]
	if !ok {
		return nil
	}
	out := make([]string, len(entry.Pubkeys))
	copy(out, entry.Pubkeys)
	return out
}

// GetAdminMutelistMeta returns the per-admin summary (count + last-synced)
// for dashboard display. Sorted by pubkey for stable rendering.
func GetAdminMutelistMeta() []AdminMutelistMeta {
	adminMutelistMu.RLock()
	defer adminMutelistMu.RUnlock()

	out := make([]AdminMutelistMeta, 0, len(adminMutelistCache))
	for pk, entry := range adminMutelistCache {
		out = append(out, AdminMutelistMeta{
			Pubkey:   pk,
			Count:    len(entry.Pubkeys),
			SyncedAt: entry.SyncedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Pubkey < out[j].Pubkey })
	return out
}

// normalizePubkeySet lowercases, trims, validates 64-char hex, and dedupes.
// Invalid entries are dropped silently — the browser is the source and has
// already produced hex from the decrypted tag arrays; a stray malformed
// entry shouldn't fail the whole sync.
func normalizePubkeySet(pubkeys []string) []string {
	seen := make(map[string]bool, len(pubkeys))
	out := make([]string, 0, len(pubkeys))
	for _, pk := range pubkeys {
		pk = strings.ToLower(strings.TrimSpace(pk))
		if len(pk) != 64 || !isLowerHexAll(pk) {
			continue
		}
		if seen[pk] {
			continue
		}
		seen[pk] = true
		out = append(out, pk)
	}
	sort.Strings(out) // stable on-disk order keeps diffs/round-trips clean
	return out
}

// isLowerHexAll reports whether s is all lowercase hex digits. Mirrors the
// client/admin.go helper of the same intent, kept local to avoid an import.
func isLowerHexAll(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func cloneAdminMutelist(m map[string]adminMutelistEntry) map[string]adminMutelistEntry {
	out := make(map[string]adminMutelistEntry, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func adminMutelistSidecarPath() string {
	dir := GetDataDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, adminMutelistSidecarFile)
}

func writeAdminMutelistSidecar(admins map[string]adminMutelistEntry) error {
	path := adminMutelistSidecarPath()
	if path == "" {
		return fmt.Errorf("data dir not set")
	}
	s := adminMutelistSidecar{Version: adminMutelistVersion, Admins: admins}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode admin mutelist sidecar: %w", err)
	}
	// Atomic write via tmp+rename so a crash mid-write can't truncate the file.
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

// ResetAdminMutelistForTest clears in-memory admin mutelist state. Tests only.
func ResetAdminMutelistForTest() {
	adminMutelistMu.Lock()
	defer adminMutelistMu.Unlock()
	adminMutelistCache = make(map[string]adminMutelistEntry)
}
