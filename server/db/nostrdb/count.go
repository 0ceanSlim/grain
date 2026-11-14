package nostrdb

import (
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// countHardCap bounds the worst-case work for a single COUNT request.
// Above this we return the cap and report `approximate: true` per NIP-45.
const countHardCap = 1_000_000

// countPageSize is how many events COUNT reads per page; a var so tests
// can cross page boundaries without storing maxQueryResults events.
var countPageSize = maxQueryResults

// CountFiltered returns the number of events matching `filters`. The
// nostrdb query API is capped at maxQueryResults per call, so we page
// backwards through created_at using the cursor pattern from the NIP-40
// bootstrap. The bool return is true when the result is approximate:
//
//   - hardCap reached,
//   - multiple filters supplied (we sum per-filter counts; a multi-filter
//     union that overlaps in event ids would double-count, and dedupe
//     would require buffering all matched ids — defeating the purpose
//     of COUNT vs REQ), or
//   - more than maxQueryResults matching events share one created_at
//     second, so a page can't hold them all and the count steps past it.
//
// Single-filter requests otherwise get an exact count.
func (db *NDB) CountFiltered(filters []nostr.Filter) (int, bool, error) {
	if len(filters) == 0 {
		return 0, false, nil
	}

	logger := log.DBQuery()
	approximate := len(filters) > 1
	total := 0

	for _, base := range filters {
		filterTotal, approx, err := countSingleFilter(db, base)
		if err != nil {
			return 0, false, err
		}
		total += filterTotal
		if approx {
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
// plus whether that count is approximate.
//
// Each full page is counted only down to, but not including, its oldest
// second. The next page starts at that second (until is inclusive), so
// events sharing a second across a page boundary are counted once.
func countSingleFilter(db *NDB, base nostr.Filter) (int, bool, error) {
	pageSize := countPageSize

	cursor := base.Until
	total := 0
	approximate := false

	for {
		limit := pageSize
		page := base
		page.Limit = &limit
		page.Until = cursor

		events, err := db.Query([]nostr.Filter{page}, pageSize)
		if err != nil {
			return 0, false, err
		}
		if len(events) < pageSize {
			total += len(events)
			break
		}

		oldestTs := events[0].CreatedAt
		for _, e := range events {
			if e.CreatedAt < oldestTs {
				oldestTs = e.CreatedAt
			}
		}
		newer := 0
		for _, e := range events {
			if e.CreatedAt > oldestTs {
				newer++
			}
		}

		next := oldestTs
		if newer == 0 {
			// The whole page is one second with more beyond it: count
			// the page and step past the second.
			total += len(events)
			approximate = true
			next = oldestTs - 1
		} else {
			total += newer
		}
		if total >= countHardCap {
			return countHardCap, true, nil
		}
		nextTime := time.Unix(next, 0)
		if base.Since != nil && nextTime.Before(*base.Since) {
			break
		}
		cursor = &nextTime
	}
	return total, approximate, nil
}
