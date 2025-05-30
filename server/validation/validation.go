package validation

import (
	"log/slog"

	"github.com/0ceanslim/grain/config"
	relay "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
)

// Result represents the outcome of a validation check
type Result struct {
	Valid bool
	Message string
}

// Set the logging component for event validation
func validationLog() *slog.Logger {
	return utils.GetLogger("event-validation")
}

// CheckBlacklistAndWhitelistCached uses cached pubkey lists for validation
func CheckBlacklistAndWhitelistCached(evt relay.Event) Result {
	// Check blacklist using cache (but still check content for word-based bans)
	if blacklisted, msg := config.CheckBlacklistCached(evt.PubKey, evt.Content); blacklisted {
		validationLog().Info("Event rejected by cached blacklist", 
			"event_id", evt.ID,
			"pubkey", evt.PubKey)
		return Result{Valid: false, Message: msg}
	}

	// Check whitelist using cache
	if isWhitelisted, msg := config.CheckWhitelistCached(evt); !isWhitelisted {
		validationLog().Info("Event rejected by cached whitelist", 
			"event_id", evt.ID,
			"pubkey", evt.PubKey)
		return Result{Valid: false, Message: msg}
	}

	return Result{Valid: true}
}

// IsPubKeyWhitelistedForSync checks cache for user sync operations
func IsPubKeyWhitelistedForSync(pubKey string) bool {
	whitelistCfg := config.GetWhitelistConfig()
	if whitelistCfg == nil {
		return false
	}

	// If whitelist is disabled, allow all
	if !whitelistCfg.PubkeyWhitelist.Enabled {
		return true
	}

	// Use cached result
	return config.GetPubkeyCache().IsWhitelisted(pubKey)
}

// CheckBlacklistAndWhitelist checks if an event is allowed by the blacklist and whitelist rules
func CheckBlacklistAndWhitelist(evt relay.Event) Result {
	if blacklisted, msg := config.CheckBlacklist(evt.PubKey, evt.Content); blacklisted {
		validationLog().Info("Event rejected by blacklist", 
			"event_id", evt.ID,
			"pubkey", evt.PubKey)
		return Result{Valid: false, Message: msg}
	}

	if isWhitelisted, msg := config.CheckWhitelist(evt); !isWhitelisted {
		validationLog().Info("Event rejected by whitelist", 
			"event_id", evt.ID,
			"pubkey", evt.PubKey)
		return Result{Valid: false, Message: msg}
	}

	return Result{Valid: true}
}

// CheckRateAndSizeLimits checks if an event passes rate and size limits
func CheckRateAndSizeLimits(evt relay.Event, eventSize int) Result {
	rateLimiter := config.GetRateLimiter()
	sizeLimiter := config.GetSizeLimiter()
	category := utils.DetermineEventCategory(evt.Kind)

	if allowed, msg := rateLimiter.AllowEvent(evt.Kind, category); !allowed {
		validationLog().Info("Event rejected by rate limiter", 
			"event_id", evt.ID,
			"kind", evt.Kind,
			"category", category)
		return Result{Valid: false, Message: msg}
	}

	if allowed, msg := sizeLimiter.AllowSize(evt.Kind, eventSize); !allowed {
		validationLog().Info("Event rejected by size limiter", 
			"event_id", evt.ID,
			"kind", evt.Kind,
			"size", eventSize)
		return Result{Valid: false, Message: msg}
	}

	return Result{Valid: true}
}