package nostrdb

import (
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// countHardCap bounds the worst-case work for a single COUNT request.
// Above this we return the cap and report `approximate: true` per NIP-45.
const countHardCap = 1_000_000

// CountFiltered returns the number of events matching `filters`. The
// nostrdb query API is capped at maxQueryResults per call, so we page
// backwards through created_at using the cursor pattern from the NIP-40
// bootstrap. The bool return is true when the result is approximate:
//
//   - hardCap reached, or
//   - multiple filters supplied (we sum per-filter counts; a multi-filter
//     union that overlaps in event ids would double-count, and dedupe
//     would require buffering all matched ids — defeating the purpose
//     of COUNT vs REQ).
//
// Single-filter requests with sane time bounds get an exact count.
//
// One known undercount edge case: if a full page (maxQueryResults
// events) shares the same created_at, the next page's cursor advances
// by one second and may skip same-second siblings beyond the page. Same
// trade-off as PurgeOldEvents and the expiration bootstrap.
func (db *NDB) CountFiltered(filters []nostr.Filter) (int, bool, error) {
	if len(filters) == 0 {
		return 0, false, nil
	}

	logger := log.GetLogger("db-count")
	approximate := len(filters) > 1
	total := 0

	for _, base := range filters {
		filterTotal, hitCap, err := countSingleFilter(db, base)
		if err != nil {
			return 0, false, err
		}
		total += filterTotal
		if hitCap {
			approximate = true
		}
		if total >= countHardCap {
			total = countHardCap
			approximate = true
			break
		}
	}

	logger.Debug("Count completed",
		"filter_count", len(filters),
		"total", total,
		"approximate", approximate)
	return total, approximate, nil
}

// countSingleFilter pages through one filter and returns its match count
// plus a flag indicating whether the hard cap was reached for this one.
func countSingleFilter(db *NDB, base nostr.Filter) (int, bool, error) {
	const pageSize = maxQueryResults

	cursor := base.Until
	total := 0

	for {
		limit := pageSize
		page := base
		page.Limit = &limit
		page.Until = cursor

		events, err := db.Query([]nostr.Filter{page}, pageSize)
		if err != nil {
			return 0, false, err
		}
		if len(events) == 0 {
			break
		}
		total += len(events)
		if total >= countHardCap {
			return countHardCap, true, nil
		}
		if len(events) < pageSize {
			break
		}

		// nostrdb returns newest-first; advance cursor to (oldest - 1)
		// to fetch the next page strictly older than this one.
		oldestTs := events[0].CreatedAt
		for _, e := range events {
			if e.CreatedAt < oldestTs {
				oldestTs = e.CreatedAt
			}
		}
		next := time.Unix(oldestTs-1, 0)
		if base.Since != nil && next.Before(*base.Since) {
			break
		}
		cursor = &next
	}
	return total, false, nil
}
