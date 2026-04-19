package validation

import (
	"github.com/0ceanslim/grain/config"
	noatr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Result represents the outcome of a validation check
type Result struct {
	Valid   bool
	Message string
}

// CheckBlacklistAndWhitelistCached uses cached pubkey lists for validation
func CheckBlacklistAndWhitelistCached(evt noatr.Event) Result {
	// Check blacklist using cache (but still check content for word-based bans)
	if blacklisted, msg := config.CheckBlacklistCached(evt.PubKey, evt.Content); blacklisted {
		log.Validation().Info("Event rejected by cached blacklist",
			"event_id", evt.ID,
			"pubkey", evt.PubKey)
		return Result{Valid: false, Message: msg}
	}

	// Check whitelist using cache
	if isWhitelisted, msg := config.CheckWhitelistCached(evt); !isWhitelisted {
		log.Validation().Info("Event rejected by cached whitelist",
			"event_id", evt.ID,
			"pubkey", evt.PubKey)
		return Result{Valid: false, Message: msg}
	}

	return Result{Valid: true}
}

// CheckRateAndSizeLimits checks if an event passes per-client rate limits
// and global size limits.
func CheckRateAndSizeLimits(client noatr.ClientInterface, evt noatr.Event, eventSize int) Result {
	sizeLimiter := config.GetSizeLimiter()
	category := utils.DetermineEventCategory(evt.Kind)

	if allowed, msg := client.AllowEvent(evt.Kind, category); !allowed {
		log.Validation().Info("Event rejected by rate limiter",
			"event_id", evt.ID,
			"kind", evt.Kind,
			"category", category)
		return Result{Valid: false, Message: msg}
	}

	if allowed, msg := sizeLimiter.AllowSize(evt.Kind, eventSize); !allowed {
		log.Validation().Info("Event rejected by size limiter",
			"event_id", evt.ID,
			"kind", evt.Kind,
			"size", eventSize)
		return Result{Valid: false, Message: msg}
	}

	return Result{Valid: true}
}
