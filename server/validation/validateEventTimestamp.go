package validation

import (
	"strings"
	"time"

	cfgType "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// DefaultMinCreatedAt is the fallback lower bound when no operator
// config specifies one. 2020-11-07 ≈ Nostr's first public proposal
// (fiatjaf's initial sketch + earliest commits to
// nostr-protocol/nips). No legitimate Nostr event can predate this,
// so it's safe to reject anything older as garbage.
var defaultMinCreatedAt = time.Date(2020, 11, 7, 0, 0, 0, 0, time.UTC).Unix()

// defaultMaxOffset is how far into the future an event is allowed
// to be when no operator config specifies a max. Tolerates client
// clock skew without accepting plausibly-malicious far-future
// timestamps. 5 min is the historical "AUTH challenge expiry"
// ballpark.
var defaultMaxOffset = 5 * time.Minute

func ValidateEventTimestamp(evt nostr.Event, cfg *cfgType.ServerConfig) bool {
	if cfg == nil {
		log.Validation().Error("Server configuration is not loaded")
		return false
	}

	now := time.Now()
	var minCreatedAt, maxCreatedAt int64

	// Dynamically calculate min_created_at based on string configuration
	if strings.HasPrefix(cfg.EventTimeConstraints.MinCreatedAtString, "now") {
		offset := strings.TrimPrefix(cfg.EventTimeConstraints.MinCreatedAtString, "now")
		duration, err := time.ParseDuration(offset)
		if err != nil {
			log.Validation().Error("Invalid time offset for min_created_at", "offset", offset, "error", err)
			minCreatedAt = defaultMinCreatedAt
		} else {
			minCreatedAt = now.Add(duration).Unix()
		}
	} else if cfg.EventTimeConstraints.MinCreatedAt == 0 {
		minCreatedAt = defaultMinCreatedAt
	} else {
		minCreatedAt = cfg.EventTimeConstraints.MinCreatedAt
	}

	// Dynamically calculate max_created_at based on string configuration
	if strings.HasPrefix(cfg.EventTimeConstraints.MaxCreatedAtString, "now") {
		offset := strings.TrimPrefix(cfg.EventTimeConstraints.MaxCreatedAtString, "now")
		duration, err := time.ParseDuration(offset)
		if err != nil {
			log.Validation().Error("Invalid time offset for max_created_at", "offset", offset, "error", err)
			maxCreatedAt = now.Add(defaultMaxOffset).Unix()
		} else {
			maxCreatedAt = now.Add(duration).Unix()
		}
	} else if cfg.EventTimeConstraints.MaxCreatedAt == 0 {
		maxCreatedAt = now.Add(defaultMaxOffset).Unix()
	} else {
		maxCreatedAt = cfg.EventTimeConstraints.MaxCreatedAt
	}

	// Check if the event's created_at timestamp falls within the allowed range
	if evt.CreatedAt < minCreatedAt || evt.CreatedAt > maxCreatedAt {
		log.Validation().Warn("Event timestamp out of range",
			"event_id", evt.ID,
			"timestamp", evt.CreatedAt,
			"min", minCreatedAt,
			"max", maxCreatedAt,
			"event_time", time.Unix(evt.CreatedAt, 0).Format(time.RFC3339),
			"min_time", time.Unix(minCreatedAt, 0).Format(time.RFC3339),
			"max_time", time.Unix(maxCreatedAt, 0).Format(time.RFC3339))
		return false
	}

	return true
}
