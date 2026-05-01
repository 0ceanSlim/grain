package validation

import (
	"strconv"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// EventExpiration returns the unix timestamp from a NIP-40 `expiration`
// tag, or (0, false) if the event has no expiration tag. Malformed values
// are treated as no-expiration so a bad tag never permanently rejects an
// otherwise-valid event — but the warning is logged for the operator.
func EventExpiration(evt nostr.Event) (int64, bool) {
	for _, tag := range evt.Tags {
		if len(tag) >= 2 && tag[0] == "expiration" {
			ts, err := strconv.ParseInt(tag[1], 10, 64)
			if err != nil {
				log.Validation().Warn("Malformed NIP-40 expiration tag",
					"event_id", evt.ID, "value", tag[1], "error", err)
				return 0, false
			}
			return ts, true
		}
	}
	return 0, false
}

// IsExpired reports whether the event carries a NIP-40 expiration that
// has already passed relative to `now` (unix seconds).
func IsExpired(evt nostr.Event, now int64) bool {
	ts, ok := EventExpiration(evt)
	if !ok {
		return false
	}
	return ts <= now
}
