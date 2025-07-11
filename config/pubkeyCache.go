package config

import (
	"fmt"
	"sync"
	"time"

	"github.com/0ceanslim/grain/client/core/tools"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// PubkeyCache manages cached pubkey lists with source tracking for whitelist and blacklist operations
type PubkeyCache struct {
	// Enhanced whitelist data with source tracking
	whitelistDirectPubkeys map[string]bool            // Direct config pubkeys
	whitelistNpubPubkeys   map[string]bool            // Converted npub pubkeys
	whitelistDomainPubkeys map[string]map[string]bool // domain -> pubkeys map
	whitelistedPubkeys     map[string]bool            // Combined all sources (for fast lookup - backward compatibility)

	// Blacklist data (unchanged)
	blacklistedPubkeys map[string]bool

	// Mutex and timing (unchanged)
	mu                       sync.RWMutex
	lastWhitelistRefresh     time.Time
	lastBlacklistRefresh     time.Time
	whitelistRefreshInterval time.Duration
	blacklistRefreshInterval time.Duration
}

// Global cache instance with new field initialization
var globalPubkeyCache = &PubkeyCache{
	whitelistDirectPubkeys: make(map[string]bool),
	whitelistNpubPubkeys:   make(map[string]bool),
	whitelistDomainPubkeys: make(map[string]map[string]bool),
	whitelistedPubkeys:     make(map[string]bool),
	blacklistedPubkeys:     make(map[string]bool),
}

// GetPubkeyCache returns the global cache instance
func GetPubkeyCache() *PubkeyCache {
	return globalPubkeyCache
}

// InitializePubkeyCache starts the cache system with initial refresh and background updates
func InitializePubkeyCache() {
	log.Config().Info("Initializing enhanced pubkey cache system")

	// Set refresh intervals from config with defaults
	whitelistCfg := GetWhitelistConfig()
	blacklistCfg := GetBlacklistConfig()

	if whitelistCfg != nil && whitelistCfg.PubkeyWhitelist.CacheRefreshMinutes > 0 {
		globalPubkeyCache.whitelistRefreshInterval = time.Duration(whitelistCfg.PubkeyWhitelist.CacheRefreshMinutes) * time.Minute
	} else {
		globalPubkeyCache.whitelistRefreshInterval = 60 * time.Minute // Default 1 hour
	}

	if blacklistCfg != nil && blacklistCfg.MutelistCacheRefreshMinutes > 0 {
		globalPubkeyCache.blacklistRefreshInterval = time.Duration(blacklistCfg.MutelistCacheRefreshMinutes) * time.Minute
	} else {
		globalPubkeyCache.blacklistRefreshInterval = 30 * time.Minute // Default 30 minutes
	}

	// Initial refresh - always cache regardless of enabled state
	globalPubkeyCache.RefreshWhitelist()
	globalPubkeyCache.RefreshBlacklist()

	// Start background refresh routines
	globalPubkeyCache.startBackgroundRefresh()

	log.Config().Info("Enhanced pubkey cache system initialized",
		"whitelist_interval_min", int(globalPubkeyCache.whitelistRefreshInterval.Minutes()),
		"blacklist_interval_min", int(globalPubkeyCache.blacklistRefreshInterval.Minutes()))
}

// RefreshWhitelist rebuilds the whitelist cache with source tracking
// Always caches all sources regardless of enabled state for sync/purge operations
func (pc *PubkeyCache) RefreshWhitelist() error {
	start := time.Now()

	// Initialize new source-specific maps
	newDirectPubkeys := make(map[string]bool)
	newNpubPubkeys := make(map[string]bool)
	newDomainPubkeys := make(map[string]map[string]bool)
	newAllPubkeys := make(map[string]bool)

	whitelistCfg := GetWhitelistConfig()
	if whitelistCfg == nil {
		log.Config().Warn("Whitelist configuration not available")
		return fmt.Errorf("whitelist configuration not available")
	}

	log.Config().Debug("Starting enhanced whitelist cache refresh")

	// Always add direct pubkeys (regardless of enabled state)
	directCount := 0
	for _, pubkey := range whitelistCfg.PubkeyWhitelist.Pubkeys {
		newDirectPubkeys[pubkey] = true
		newAllPubkeys[pubkey] = true
		directCount++
	}

	// Always decode and add npubs (regardless of enabled state)
	npubCount := 0
	for _, npub := range whitelistCfg.PubkeyWhitelist.Npubs {
		pubkey, err := tools.DecodeNpub(npub)
		if err != nil {
			log.Config().Error("Failed to decode npub", "npub", npub, "error", err)
			continue
		}
		newNpubPubkeys[pubkey] = true
		newAllPubkeys[pubkey] = true
		npubCount++
	}

	// Always fetch domain pubkeys (regardless of enabled state)
	totalDomainCount := 0
	if len(whitelistCfg.DomainWhitelist.Domains) > 0 {
		for _, domain := range whitelistCfg.DomainWhitelist.Domains {
			domainPubkeys, err := utils.FetchPubkeysFromDomains([]string{domain})
			if err != nil {
				log.Config().Error("Failed to fetch domain pubkeys", "domain", domain, "error", err)
				// Initialize empty map for failed domain
				newDomainPubkeys[domain] = make(map[string]bool)
				continue
			}

			// Create domain-specific map
			newDomainPubkeys[domain] = make(map[string]bool)
			for _, pubkey := range domainPubkeys {
				newDomainPubkeys[domain][pubkey] = true
				newAllPubkeys[pubkey] = true // Keep for validation purposes
				totalDomainCount++
			}

			log.Config().Debug("Cached domain pubkeys",
				"domain", domain,
				"pubkey_count", len(domainPubkeys))
		}
	}

	// Update cache atomically
	pc.mu.Lock()
	pc.whitelistDirectPubkeys = newDirectPubkeys
	pc.whitelistNpubPubkeys = newNpubPubkeys
	pc.whitelistDomainPubkeys = newDomainPubkeys
	pc.whitelistedPubkeys = newAllPubkeys // Backward compatibility
	pc.lastWhitelistRefresh = time.Now()
	pc.mu.Unlock()

	duration := time.Since(start)
	log.Config().Info("Enhanced whitelist cache refreshed",
		"duration_ms", duration.Milliseconds(),
		"total_pubkeys", len(newAllPubkeys),
		"direct_pubkeys", directCount,
		"npub_pubkeys", npubCount,
		"domain_pubkeys", totalDomainCount,
		"domains_processed", len(whitelistCfg.DomainWhitelist.Domains),
		"pubkey_enabled", whitelistCfg.PubkeyWhitelist.Enabled,
		"domain_enabled", whitelistCfg.DomainWhitelist.Enabled)

	return nil
}

// GetWhitelistedPubkeys returns a copy of all whitelisted pubkeys for bulk operations
// Maintains backward compatibility
func (pc *PubkeyCache) GetWhitelistedPubkeys() []string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	result := make([]string, 0, len(pc.whitelistedPubkeys))
	for pubkey := range pc.whitelistedPubkeys {
		result = append(result, pubkey)
	}
	return result
}

// GetDirectWhitelistedPubkeys returns only direct config pubkeys (excluding domain pubkeys)
// Use this for API endpoints that want to show only directly configured pubkeys
func (pc *PubkeyCache) GetDirectWhitelistedPubkeys() []string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	result := make([]string, 0, len(pc.whitelistDirectPubkeys)+len(pc.whitelistNpubPubkeys))

	// Add direct pubkeys
	for pubkey := range pc.whitelistDirectPubkeys {
		result = append(result, pubkey)
	}

	// Add npub pubkeys
	for pubkey := range pc.whitelistNpubPubkeys {
		result = append(result, pubkey)
	}

	return result
}

// GetDomainPubkeys returns pubkeys for a specific domain from cache
func (pc *PubkeyCache) GetDomainPubkeys(domain string) []string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	domainPubkeys, exists := pc.whitelistDomainPubkeys[domain]
	if !exists {
		return []string{}
	}

	result := make([]string, 0, len(domainPubkeys))
	for pubkey := range domainPubkeys {
		result = append(result, pubkey)
	}
	return result
}

// GetWhitelistSourceBreakdown returns detailed source breakdown for API endpoints
func (pc *PubkeyCache) GetWhitelistSourceBreakdown() map[string]interface{} {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	breakdown := map[string]interface{}{
		"direct_count": len(pc.whitelistDirectPubkeys),
		"npub_count":   len(pc.whitelistNpubPubkeys),
		"domain_count": len(pc.whitelistDomainPubkeys),
		"total_count":  len(pc.whitelistedPubkeys),
	}

	// Add domain-specific counts
	domainCounts := make(map[string]int)
	for domain, pubkeys := range pc.whitelistDomainPubkeys {
		domainCounts[domain] = len(pubkeys)
	}
	breakdown["domains"] = domainCounts

	return breakdown
}

// IsWhitelisted checks if a pubkey is in ANY whitelist source (fast lookup)
// Maintains backward compatibility
func (pc *PubkeyCache) IsWhitelisted(pubkey string) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.whitelistedPubkeys[pubkey]
}

// IsWhitelistedForValidation checks if a pubkey is whitelisted AND whitelist is enabled
// Maintains backward compatibility
func (pc *PubkeyCache) IsWhitelistedForValidation(pubkey string) bool {
	whitelistCfg := GetWhitelistConfig()
	if whitelistCfg == nil {
		return false
	}

	// If pubkey whitelist is disabled, all pubkeys are considered valid
	if !whitelistCfg.PubkeyWhitelist.Enabled {
		return true
	}

	return pc.IsWhitelisted(pubkey)
}

// Blacklist functions remain unchanged for backward compatibility
func (pc *PubkeyCache) RefreshBlacklist() error {
	start := time.Now()
	newBlacklist := make(map[string]bool)

	blacklistCfg := GetBlacklistConfig()
	if blacklistCfg == nil {
		log.Config().Debug("Blacklist configuration not available")
		// Don't return error - just cache empty list
		pc.mu.Lock()
		pc.blacklistedPubkeys = newBlacklist
		pc.lastBlacklistRefresh = time.Now()
		pc.mu.Unlock()
		return nil
	}

	log.Config().Debug("Starting blacklist cache refresh")

	// Always add permanent banned pubkeys (regardless of enabled state)
	directCount := 0
	for _, pubkey := range blacklistCfg.PermanentBlacklistPubkeys {
		newBlacklist[pubkey] = true
		directCount++
	}

	// Always decode and add banned npubs (regardless of enabled state)
	npubCount := 0
	for _, npub := range blacklistCfg.PermanentBlacklistNpubs {
		pubkey, err := tools.DecodeNpub(npub)
		if err != nil {
			log.Config().Error("Failed to decode blacklisted npub", "npub", npub, "error", err)
			continue
		}
		newBlacklist[pubkey] = true
		npubCount++
	}

	// Always fetch mutelist pubkeys (regardless of enabled state)
	mutelistCount := 0
	if len(blacklistCfg.MuteListAuthors) > 0 {
		serverCfg := GetConfig()
		if serverCfg != nil {
			localRelayURL := fmt.Sprintf("ws://localhost%s", serverCfg.Server.Port)
			mutelistPubkeys, err := FetchPubkeysFromLocalMuteList(localRelayURL, blacklistCfg.MuteListAuthors)
			if err != nil {
				log.Config().Error("Failed to fetch mutelist pubkeys", "error", err)
			} else {
				for _, pubkey := range mutelistPubkeys {
					newBlacklist[pubkey] = true
					mutelistCount++
				}
			}
		}
	}

	// Update cache atomically
	pc.mu.Lock()
	pc.blacklistedPubkeys = newBlacklist
	pc.lastBlacklistRefresh = time.Now()
	pc.mu.Unlock()

	duration := time.Since(start)
	log.Config().Info("Blacklist cache refreshed",
		"duration_ms", duration.Milliseconds(),
		"total_pubkeys", len(newBlacklist),
		"direct_pubkeys", directCount,
		"npub_pubkeys", npubCount,
		"mutelist_pubkeys", mutelistCount,
		"blacklist_enabled", blacklistCfg.Enabled)

	return nil
}

// Blacklist functions remain unchanged
func (pc *PubkeyCache) IsBlacklisted(pubkey string) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.blacklistedPubkeys[pubkey]
}

func (pc *PubkeyCache) IsBlacklistedForValidation(pubkey string) bool {
	blacklistCfg := GetBlacklistConfig()
	if blacklistCfg == nil || !blacklistCfg.Enabled {
		return false
	}

	return pc.IsBlacklisted(pubkey)
}

func (pc *PubkeyCache) GetBlacklistedPubkeys() []string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	result := make([]string, 0, len(pc.blacklistedPubkeys))
	for pubkey := range pc.blacklistedPubkeys {
		result = append(result, pubkey)
	}
	return result
}

// GetPubkeyCacheStats returns enhanced cache statistics for monitoring
func (pc *PubkeyCache) GetPubkeyCacheStats() map[string]interface{} {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	whitelistCfg := GetWhitelistConfig()
	blacklistCfg := GetBlacklistConfig()

	stats := map[string]interface{}{
		"whitelist_count":        len(pc.whitelistedPubkeys),
		"whitelist_direct_count": len(pc.whitelistDirectPubkeys),
		"whitelist_npub_count":   len(pc.whitelistNpubPubkeys),
		"whitelist_domain_count": len(pc.whitelistDomainPubkeys),
		"blacklist_count":        len(pc.blacklistedPubkeys),
		"last_whitelist_refresh": pc.lastWhitelistRefresh.Format(time.RFC3339),
		"last_blacklist_refresh": pc.lastBlacklistRefresh.Format(time.RFC3339),
		"whitelist_age_minutes":  time.Since(pc.lastWhitelistRefresh).Minutes(),
		"blacklist_age_minutes":  time.Since(pc.lastBlacklistRefresh).Minutes(),
	}

	// Add enabled state information
	if whitelistCfg != nil {
		stats["pubkey_whitelist_enabled"] = whitelistCfg.PubkeyWhitelist.Enabled
		stats["domain_whitelist_enabled"] = whitelistCfg.DomainWhitelist.Enabled
	}
	if blacklistCfg != nil {
		stats["blacklist_enabled"] = blacklistCfg.Enabled
	}

	// Add domain breakdown
	domainCounts := make(map[string]int)
	for domain, pubkeys := range pc.whitelistDomainPubkeys {
		domainCounts[domain] = len(pubkeys)
	}
	stats["domain_breakdown"] = domainCounts

	return stats
}

// startBackgroundRefresh starts goroutines for periodic cache refresh
func (pc *PubkeyCache) startBackgroundRefresh() {
	// Whitelist refresh routine
	go func() {
		ticker := time.NewTicker(pc.whitelistRefreshInterval)
		defer ticker.Stop()

		for range ticker.C {
			if err := pc.RefreshWhitelist(); err != nil {
				log.Config().Error("Failed to refresh whitelist cache", "error", err)
			}
		}
	}()

	// Blacklist refresh routine
	go func() {
		ticker := time.NewTicker(pc.blacklistRefreshInterval)
		defer ticker.Stop()

		for range ticker.C {
			if err := pc.RefreshBlacklist(); err != nil {
				log.Config().Error("Failed to refresh blacklist cache", "error", err)
			}
		}
	}()

	log.Config().Info("Background cache refresh routines started")
}
