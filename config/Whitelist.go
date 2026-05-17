package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/0ceanslim/grain/client/core/tools"
	cfgType "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"gopkg.in/yaml.v3"
)

// CheckWhitelistCached uses cached pubkey lists and respects enabled state for validation
func CheckWhitelistCached(evt nostr.Event) (bool, string) {
	whitelistCfg := GetWhitelistConfig()
	if whitelistCfg == nil {
		log.Config().Error("Whitelist configuration is missing")
		return false, "Internal server error: whitelist configuration is missing"
	}

	// Check if the event's kind is whitelisted (no caching needed for this)
	if whitelistCfg.KindWhitelist.Enabled && !IsKindWhitelisted(evt.Kind) {
		log.Config().Warn("Event kind is not whitelisted", "kind", evt.Kind)
		return false, "not allowed: event kind is not whitelisted"
	}

	// Check if the event's pubkey is whitelisted using cache with enabled state check
	pubkeyCache := GetPubkeyCache()
	if whitelistCfg.PubkeyWhitelist.Enabled && !pubkeyCache.IsWhitelistedForValidation(evt.PubKey) {
		log.Config().Warn("Pubkey is not whitelisted", "pubkey", evt.PubKey)
		return false, "not allowed: pubkey or npub is not whitelisted"
	}

	log.Config().Debug("Whitelist check passed", "kind", evt.Kind, "pubkey", evt.PubKey)
	return true, ""
}

// IsPubKeyWhitelistedCached for purging operations - always uses cache regardless of enabled state
func IsPubKeyWhitelistedCached(pubKey string, skipEnabledCheck bool) bool {
	pubkeyCache := GetPubkeyCache()

	if skipEnabledCheck {
		// For purging operations - use cache regardless of enabled state
		return pubkeyCache.IsWhitelisted(pubKey)
	}

	// For validation operations - respect enabled state
	return pubkeyCache.IsWhitelistedForValidation(pubKey)
}

// AddPubkeyToWhitelist appends pubkey to whitelist.yml's
// pubkey_whitelist.pubkeys. Used by NIP-86 `allowpubkey`. The
// configured set is grain's "elevated users" registry regardless
// of whether the whitelist is currently enabled — see the
// [[project-whitelist-semantics]] note.
//
// Caller doesn't need to acquire ConfigMu; this function does it
// internally. Idempotent: returns nil with an info log if the
// pubkey is already in either the hex or npub list.
func AddPubkeyToWhitelist(pubkey string) error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()

	cfg := GetWhitelistConfig()
	if cfg == nil {
		return fmt.Errorf("whitelist configuration is not loaded")
	}

	lower := strings.ToLower(pubkey)
	for _, p := range cfg.PubkeyWhitelist.Pubkeys {
		if strings.ToLower(p) == lower {
			log.Config().Info("Pubkey already in whitelist, no-op", "pubkey", pubkey)
			return nil
		}
	}

	cfg.PubkeyWhitelist.Pubkeys = append(cfg.PubkeyWhitelist.Pubkeys, pubkey)
	log.Config().Info("Added pubkey to whitelist", "pubkey", pubkey)
	return saveWhitelistConfig(*cfg)
}

// RemovePubkeyFromWhitelist removes pubkey from both
// pubkey_whitelist.pubkeys and pubkey_whitelist.npubs (decoded
// npubs matched against the supplied hex). Used by NIP-86
// `unallowpubkey`. Idempotent — removing a pubkey that isn't there
// is a no-op.
func RemovePubkeyFromWhitelist(pubkey string) error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()

	cfg := GetWhitelistConfig()
	if cfg == nil {
		return fmt.Errorf("whitelist configuration is not loaded")
	}

	lower := strings.ToLower(pubkey)
	origPubkeys := cfg.PubkeyWhitelist.Pubkeys
	origNpubs := cfg.PubkeyWhitelist.Npubs

	keptPubkeys := make([]string, 0, len(origPubkeys))
	for _, p := range origPubkeys {
		if strings.ToLower(p) != lower {
			keptPubkeys = append(keptPubkeys, p)
		}
	}

	// Same defensive npub handling as the blacklist remove:
	// keep malformed entries instead of silently dropping them.
	keptNpubs := make([]string, 0, len(origNpubs))
	for _, n := range origNpubs {
		decoded, err := decodeNpubSafe(n)
		if err != nil || strings.ToLower(decoded) != lower {
			keptNpubs = append(keptNpubs, n)
		}
	}

	if len(keptPubkeys) == len(origPubkeys) && len(keptNpubs) == len(origNpubs) {
		log.Config().Info("Pubkey not in whitelist, no-op", "pubkey", pubkey)
		return nil
	}

	cfg.PubkeyWhitelist.Pubkeys = keptPubkeys
	cfg.PubkeyWhitelist.Npubs = keptNpubs
	log.Config().Info("Removed pubkey from whitelist", "pubkey", pubkey)
	return saveWhitelistConfig(*cfg)
}

// AddKindToWhitelist appends kind to whitelist.yml's
// kind_whitelist.kinds. Used by NIP-86 `allowkind`. The yaml field
// is `[]string` so older operator-edited configs that store kind
// labels or ranges still parse; this helper stores ints via
// strconv.Itoa. Idempotent.
func AddKindToWhitelist(kind int) error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()

	cfg := GetWhitelistConfig()
	if cfg == nil {
		return fmt.Errorf("whitelist configuration is not loaded")
	}

	target := strconv.Itoa(kind)
	for _, existing := range cfg.KindWhitelist.Kinds {
		// Compare both as parsed ints (so "0001" matches 1) and
		// as raw strings (so labels survive verbatim if added
		// later).
		if existing == target {
			log.Config().Info("Kind already in whitelist, no-op", "kind", kind)
			return nil
		}
		if n, err := strconv.Atoi(existing); err == nil && n == kind {
			log.Config().Info("Kind already in whitelist, no-op", "kind", kind)
			return nil
		}
	}

	cfg.KindWhitelist.Kinds = append(cfg.KindWhitelist.Kinds, target)
	log.Config().Info("Added kind to whitelist", "kind", kind)
	return saveWhitelistConfig(*cfg)
}

// RemoveKindFromWhitelist removes any entry that parses to the
// given kind. Used by NIP-86 `disallowkind`. Idempotent.
func RemoveKindFromWhitelist(kind int) error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()

	cfg := GetWhitelistConfig()
	if cfg == nil {
		return fmt.Errorf("whitelist configuration is not loaded")
	}

	orig := cfg.KindWhitelist.Kinds
	kept := make([]string, 0, len(orig))
	for _, s := range orig {
		if n, err := strconv.Atoi(s); err == nil && n == kind {
			continue
		}
		kept = append(kept, s)
	}

	if len(kept) == len(orig) {
		log.Config().Info("Kind not in whitelist, no-op", "kind", kind)
		return nil
	}

	cfg.KindWhitelist.Kinds = kept
	log.Config().Info("Removed kind from whitelist", "kind", kind)
	return saveWhitelistConfig(*cfg)
}

// saveWhitelistConfig is the whitelist twin of saveBlacklistConfig:
// suppress the watcher, atomically write whitelist.yml, refresh the
// in-memory cache. Caller MUST hold ConfigMu.
func saveWhitelistConfig(cfg cfgType.WhitelistConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal whitelist config: %v", err)
	}

	path := ConfigPath("whitelist.yml")
	SuppressWatcherFor(path)

	if err := AtomicWriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write whitelist.yml: %v", err)
	}

	if cache := GetPubkeyCache(); cache != nil {
		if err := cache.RefreshWhitelist(); err != nil {
			log.Config().Warn("Failed to refresh whitelist cache after write", "error", err)
		}
	}
	return nil
}

// decodeNpubSafe wraps tools.DecodeNpub with an extra guard for the
// rare case where it returns "" with no error (defensive — should
// never happen, but the whitelist npub path runs on user-supplied
// data so a bad return there shouldn't silently match an empty
// string against any pubkey).
func decodeNpubSafe(npub string) (string, error) {
	decoded, err := tools.DecodeNpub(npub)
	if err != nil {
		return "", err
	}
	if decoded == "" {
		return "", fmt.Errorf("empty decode result")
	}
	return decoded, nil
}

// Check if a kind is whitelisted
func IsKindWhitelisted(kind int) bool {
	cfg := GetWhitelistConfig()
	if !cfg.KindWhitelist.Enabled {
		return true
	}

	for _, whitelistedKindStr := range cfg.KindWhitelist.Kinds {
		whitelistedKind, err := strconv.Atoi(whitelistedKindStr)
		if err != nil {
			log.Config().Error("Failed to convert whitelisted kind to int", "kind", whitelistedKindStr, "error", err)
			continue
		}
		if kind == whitelistedKind {
			return true
		}
	}

	return false
}
