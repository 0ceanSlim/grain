package config

import (
	"fmt"
	"sync"
	"time"

	"github.com/0ceanslim/grain/server/utils"
)

// PubkeyCache manages cached pubkey lists for whitelist and blacklist operations
type PubkeyCache struct {
	whitelistedPubkeys       map[string]bool
	blacklistedPubkeys       map[string]bool
	mu                       sync.RWMutex
	lastWhitelistRefresh     time.Time
	lastBlacklistRefresh     time.Time
	whitelistRefreshInterval time.Duration
	blacklistRefreshInterval time.Duration
}

// Global cache instance
var globalPubkeyCache = &PubkeyCache{
	whitelistedPubkeys: make(map[string]bool),
	blacklistedPubkeys: make(map[string]bool),
}

// GetPubkeyCache returns the global cache instance
func GetPubkeyCache() *PubkeyCache {
	return globalPubkeyCache
}

// InitializePubkeyCache starts the cache system with initial refresh and background updates
func InitializePubkeyCache() {
	configLog().Info("Initializing pubkey cache system")
	
	// Set refresh intervals from config
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
	
	// Initial refresh
	globalPubkeyCache.RefreshWhitelist()
	globalPubkeyCache.RefreshBlacklist()
	
	// Start background refresh routines
	globalPubkeyCache.startBackgroundRefresh()
	
	configLog().Info("Pubkey cache system initialized",
		"whitelist_interval_min", int(globalPubkeyCache.whitelistRefreshInterval.Minutes()),
		"blacklist_interval_min", int(globalPubkeyCache.blacklistRefreshInterval.Minutes()))
}

// RefreshWhitelist rebuilds the whitelist cache from configuration
func (pc *PubkeyCache) RefreshWhitelist() error {
	start := time.Now()
	newWhitelist := make(map[string]bool)
	
	whitelistCfg := GetWhitelistConfig()
	if whitelistCfg == nil {
		configLog().Warn("Whitelist configuration not available")
		return fmt.Errorf("whitelist configuration not available")
	}
	
	configLog().Debug("Starting whitelist cache refresh")
	
	// Add direct pubkeys
	for _, pubkey := range whitelistCfg.PubkeyWhitelist.Pubkeys {
		newWhitelist[pubkey] = true
	}
	directCount := len(whitelistCfg.PubkeyWhitelist.Pubkeys)
	
	// Decode and add npubs
	npubCount := 0
	for _, npub := range whitelistCfg.PubkeyWhitelist.Npubs {
		pubkey, err := utils.DecodeNpub(npub)
		if err != nil {
			configLog().Error("Failed to decode npub", "npub", npub, "error", err)
			continue
		}
		newWhitelist[pubkey] = true
		npubCount++
	}
	
	// Fetch domain pubkeys if enabled
	domainCount := 0
	if whitelistCfg.DomainWhitelist.Enabled && len(whitelistCfg.DomainWhitelist.Domains) > 0 {
		domainPubkeys, err := utils.FetchPubkeysFromDomains(whitelistCfg.DomainWhitelist.Domains)
		if err != nil {
			configLog().Error("Failed to fetch domain pubkeys", "error", err)
		} else {
			for _, pubkey := range domainPubkeys {
				newWhitelist[pubkey] = true
				domainCount++
			}
		}
	}
	
	// Update cache atomically
	pc.mu.Lock()
	pc.whitelistedPubkeys = newWhitelist
	pc.lastWhitelistRefresh = time.Now()
	pc.mu.Unlock()
	
	duration := time.Since(start)
	configLog().Info("Whitelist cache refreshed",
		"duration_ms", duration.Milliseconds(),
		"total_pubkeys", len(newWhitelist),
		"direct_pubkeys", directCount,
		"npub_pubkeys", npubCount,
		"domain_pubkeys", domainCount)
	
	return nil
}

// RefreshBlacklist rebuilds the blacklist cache from configuration
func (pc *PubkeyCache) RefreshBlacklist() error {
	start := time.Now()
	newBlacklist := make(map[string]bool)
	
	blacklistCfg := GetBlacklistConfig()
	if blacklistCfg == nil || !blacklistCfg.Enabled {
		configLog().Debug("Blacklist configuration not available or disabled")
		return fmt.Errorf("blacklist configuration not available or disabled")
	}
	
	configLog().Debug("Starting blacklist cache refresh")
	
	// Add permanent banned pubkeys
	for _, pubkey := range blacklistCfg.PermanentBlacklistPubkeys {
		newBlacklist[pubkey] = true
	}
	directCount := len(blacklistCfg.PermanentBlacklistPubkeys)
	
	// Decode and add banned npubs
	npubCount := 0
	for _, npub := range blacklistCfg.PermanentBlacklistNpubs {
		pubkey, err := utils.DecodeNpub(npub)
		if err != nil {
			configLog().Error("Failed to decode blacklisted npub", "npub", npub, "error", err)
			continue
		}
		newBlacklist[pubkey] = true
		npubCount++
	}
	
	// Fetch mutelist pubkeys if configured
	mutelistCount := 0
	if len(blacklistCfg.MuteListAuthors) > 0 {
		serverCfg := GetConfig()
		if serverCfg != nil {
			localRelayURL := fmt.Sprintf("ws://localhost%s", serverCfg.Server.Port)
			mutelistPubkeys, err := FetchPubkeysFromLocalMuteList(localRelayURL, blacklistCfg.MuteListAuthors)
			if err != nil {
				configLog().Error("Failed to fetch mutelist pubkeys", "error", err)
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
	configLog().Info("Blacklist cache refreshed",
		"duration_ms", duration.Milliseconds(),
		"total_pubkeys", len(newBlacklist),
		"direct_pubkeys", directCount,
		"npub_pubkeys", npubCount,
		"mutelist_pubkeys", mutelistCount)
	
	return nil
}

// IsWhitelisted checks if a pubkey is in the whitelist cache
func (pc *PubkeyCache) IsWhitelisted(pubkey string) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.whitelistedPubkeys[pubkey]
}

// IsBlacklisted checks if a pubkey is in the blacklist cache
func (pc *PubkeyCache) IsBlacklisted(pubkey string) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.blacklistedPubkeys[pubkey]
}

// GetWhitelistedPubkeys returns a copy of all whitelisted pubkeys for bulk operations
func (pc *PubkeyCache) GetWhitelistedPubkeys() []string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	
	result := make([]string, 0, len(pc.whitelistedPubkeys))
	for pubkey := range pc.whitelistedPubkeys {
		result = append(result, pubkey)
	}
	return result
}

// GetBlacklistedPubkeys returns a copy of all blacklisted pubkeys for bulk operations
func (pc *PubkeyCache) GetBlacklistedPubkeys() []string {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	
	result := make([]string, 0, len(pc.blacklistedPubkeys))
	for pubkey := range pc.blacklistedPubkeys {
		result = append(result, pubkey)
	}
	return result
}

// GetPubkeyCacheStats returns cache statistics for monitoring
func (pc *PubkeyCache) GetPubkeyCacheStats() map[string]interface{} {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	
	return map[string]interface{}{
		"whitelist_count":        len(pc.whitelistedPubkeys),
		"blacklist_count":        len(pc.blacklistedPubkeys),
		"last_whitelist_refresh": pc.lastWhitelistRefresh.Format(time.RFC3339),
		"last_blacklist_refresh": pc.lastBlacklistRefresh.Format(time.RFC3339),
		"whitelist_age_minutes":  time.Since(pc.lastWhitelistRefresh).Minutes(),
		"blacklist_age_minutes":  time.Since(pc.lastBlacklistRefresh).Minutes(),
	}
}

// startBackgroundRefresh starts goroutines for periodic cache refresh
func (pc *PubkeyCache) startBackgroundRefresh() {
	// Whitelist refresh routine
	go func() {
		ticker := time.NewTicker(pc.whitelistRefreshInterval)
		defer ticker.Stop()
		
		for range ticker.C {
			if err := pc.RefreshWhitelist(); err != nil {
				configLog().Error("Failed to refresh whitelist cache", "error", err)
			}
		}
	}()
	
	// Blacklist refresh routine
	go func() {
		ticker := time.NewTicker(pc.blacklistRefreshInterval)
		defer ticker.Stop()
		
		for range ticker.C {
			if err := pc.RefreshBlacklist(); err != nil {
				configLog().Error("Failed to refresh blacklist cache", "error", err)
			}
		}
	}()
	
	configLog().Info("Background cache refresh routines started")
}