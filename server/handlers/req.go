package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/db/nostrdb"
	"github.com/0ceanslim/grain/server/handlers/response"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
	"github.com/0ceanslim/grain/server/validation"
)

// HandleReq processes a new subscription request with proper subscription management
func HandleReq(client nostr.ClientInterface, message []interface{}) {
	if len(message) < 3 {
		log.Req().Error("Invalid REQ message format")
		response.SendClosed(client, "", "invalid: invalid REQ message format")
		return
	}

	subID, ok := message[1].(string)

	// Enforce NIP-42 authentication if required
	cfg := config.GetConfig()
	if cfg.Auth.Required {
		if !IsAuthenticated(client) {
			log.Req().Info("REQ rejected: authentication required", "sub_id", subID)
			response.SendClosed(client, subID, "auth-required: authentication is required to use this relay")
			return
		}
	}

	if !ok || len(subID) == 0 || len(subID) > 64 {
		log.Req().Error("Invalid subscription ID format or length",
			"sub_id", subID,
			"length", len(subID))
		response.SendClosed(client, "", "invalid: subscription ID must be between 1 and 64 characters long")
		return
	}

	// Per-client REQ rate limiting
	if allowed, msg := client.AllowReq(); !allowed {
		log.Req().Warn("REQ rate limit exceeded",
			"sub_id", subID,
			"reason", msg)
		response.SendClosed(client, subID, "rate-limited: "+msg)
		return
	}

	// Parse and validate filters
	filters := make([]nostr.Filter, len(message)-2)
	for i, filter := range message[2:] {
		filterData, ok := filter.(map[string]interface{})
		if !ok {
			log.Req().Error("Invalid filter format",
				"sub_id", subID,
				"filter_index", i)
			response.SendClosed(client, subID, "invalid: invalid filter format")
			return
		}

		var f nostr.Filter
		f.IDs = utils.ToStringArray(filterData["ids"])
		f.Authors = utils.ToStringArray(filterData["authors"])
		f.Kinds = utils.ToIntArray(filterData["kinds"])
		f.Since = utils.ToTime(filterData["since"])
		f.Until = utils.ToTime(filterData["until"])
		f.Limit = utils.ToInt(filterData["limit"])
		// NIP-50: optional fulltext search query.
		if s, ok := filterData["search"].(string); ok {
			f.Search = s
		}

		// NIP-01: tag filters are top-level keys like "#e", "#p", "#a".
		f.Tags = make(map[string][]string)
		for k, v := range filterData {
			if len(k) >= 2 && k[0] == '#' {
				tagName := k[1:]
				if vals := utils.ToStringArray(v); len(vals) > 0 {
					f.Tags[tagName] = vals
				}
			}
		}

		filters[i] = f
	}

	// Check if this is a duplicate subscription (same filters)
	subscriptions := client.GetSubscriptions()
	if existingFilters, exists := subscriptions[subID]; exists {
		if areFiltersIdentical(existingFilters, filters) {
			log.Req().Debug("Duplicate subscription detected, ignoring",
				"sub_id", subID,
				"filter_count", len(filters))
			// Still send EOSE for duplicate subscriptions to satisfy client expectations.
			// Use the blocking variant for symmetry with the historical-fulfillment
			// path below — there's only one frame here, so backpressure is moot, but
			// the error return lets us bail cleanly if the client has already gone.
			_ = client.SendMessageBlocking([]interface{}{"EOSE", subID})
			return
		} else {
			log.Req().Info("Subscription updated with new filters",
				"sub_id", subID,
				"old_filter_count", len(existingFilters),
				"new_filter_count", len(filters))
		}
	}

	// Remove oldest subscription if needed
	subCount := client.SubscriptionCount()
	if subCount >= config.GetConfig().Server.MaxSubscriptionsPerClient {
		for id := range subscriptions {
			if id != subID {
				client.DeleteSubscription(id)
				log.Req().Info("Dropped oldest subscription",
					"old_sub_id", id,
					"current_count", subCount-1)
				break
			}
		}
	}

	// Add/update subscription - stays active after EOSE
	client.SetSubscription(subID, filters)
	log.Req().Info("Subscription created/updated",
		"sub_id", subID,
		"filter_count", len(filters),
		"total_subscriptions", client.SubscriptionCount())

	// Query database for historical events
	db := nostrdb.GetDB()
	if db == nil {
		log.Req().Error("Database not available", "sub_id", subID)
		response.SendClosed(client, subID, "error: database not available")
		return
	}

	// Determine effective limit from config
	effectiveLimit := 1000
	if cfg := config.GetConfig(); cfg != nil && cfg.Server.ImplicitReqLimit > 0 {
		effectiveLimit = cfg.Server.ImplicitReqLimit
	}

	// Split filters into search vs. non-search. NIP-50 search filters
	// hit nostrdb's fulltext index via TextSearch; the rest go through
	// the standard Query path. Concatenate results — NIP-01 multi-
	// filter REQs are already a union with no dedupe contract.
	var nonSearch []nostr.Filter
	var searchFilters []nostr.Filter
	for _, f := range filters {
		if f.Search != "" {
			searchFilters = append(searchFilters, f)
		} else {
			nonSearch = append(nonSearch, f)
		}
	}

	var queriedEvents []nostr.Event
	if len(nonSearch) > 0 {
		evts, err := db.Query(nonSearch, effectiveLimit)
		if err != nil {
			log.Req().Error("Error querying events",
				"sub_id", subID,
				"error", err)
			response.SendClosed(client, subID, "error: could not query events")
			return
		}
		queriedEvents = append(queriedEvents, evts...)
	}
	for _, sf := range searchFilters {
		evts, err := pagedTextSearch(db, sf, effectiveLimit)
		if err != nil {
			log.Req().Error("Error executing search",
				"sub_id", subID,
				"error", err)
			response.SendClosed(client, subID, "error: could not run search")
			return
		}
		queriedEvents = append(queriedEvents, evts...)
	}

	// Send historical events to client. Use SendMessageBlocking so the
	// producer (this loop) stays in step with the writeLoop consumer —
	// without backpressure, a 500-event REQ would shove all 500 events
	// into the per-client buffer faster than the WS write rate could
	// drain, overflow the channel, and hit the slow-consumer guard,
	// disconnecting a perfectly healthy client mid-fulfillment. See
	// the SendMessageBlocking docstring for the full pathology.
	// NIP-40: drop events whose expiration has passed. Defense in depth
	// alongside the background sweeper — guarantees expired events are
	// never served even if a sweep hasn't yet reached them.
	nowUnix := time.Now().Unix()
	delivered := 0
	skippedExpired := 0
	aborted := false
	for _, evt := range queriedEvents {
		if validation.IsExpired(evt, nowUnix) {
			skippedExpired++
			continue
		}
		if err := client.SendMessageBlocking([]interface{}{"EVENT", subID, evt}); err != nil {
			// Client gone; skip the rest and the EOSE. The
			// "Subscription established" log below will still
			// fire so we have a record of how many made it.
			log.Req().Debug("REQ fulfillment aborted: client disconnected",
				"sub_id", subID,
				"delivered", delivered,
				"total_queried", len(queriedEvents))
			aborted = true
			break
		}
		delivered++
	}
	if !aborted {
		// Only send EOSE if we delivered the whole historical batch
		// (expired-skipped events count as delivered for EOSE purposes).
		_ = client.SendMessageBlocking([]interface{}{"EOSE", subID})
	}

	log.Req().Info("Subscription established",
		"sub_id", subID,
		"historical_events_sent", delivered,
		"skipped_expired", skippedExpired,
		"status", "active")

	// NOTE: Subscription remains ACTIVE after EOSE
	// It will be closed only when:
	// 1. Client sends CLOSE message
	// 2. Client disconnects
	// 3. New REQ with same subID (replaces this one)
	// 4. Subscription limit reached (oldest removed)
}

// areFiltersIdentical compares two filter slices to detect duplicates
func areFiltersIdentical(filters1, filters2 []nostr.Filter) bool {
	if len(filters1) != len(filters2) {
		return false
	}

	// Simple approach: serialize both and compare hashes
	hash1 := hashFilters(filters1)
	hash2 := hashFilters(filters2)

	return hash1 == hash2
}

// pagedTextSearch runs the NIP-50 search on `f` and pages through the
// nostrdb 128-result-per-call cap until either the effective REQ limit
// is filled, the search is exhausted, or the filter's Since bound is
// crossed. Same Until-cursor pattern as CountFiltered and the
// expiration bootstrap; same same-second-tie undercount caveat.
func pagedTextSearch(db *nostrdb.NDB, f nostr.Filter, effectiveLimit int) ([]nostr.Event, error) {
	const pageSize = 128
	remaining := effectiveLimit
	if f.Limit != nil && *f.Limit > 0 && *f.Limit < remaining {
		remaining = *f.Limit
	}

	var acc []nostr.Event
	cursor := f.Until
	for remaining > 0 {
		page := f
		page.Until = cursor

		want := pageSize
		if remaining < want {
			want = remaining
		}
		events, err := db.TextSearch(f.Search, page, want)
		if err != nil {
			return nil, err
		}
		if len(events) == 0 {
			break
		}
		acc = append(acc, events...)
		remaining -= len(events)

		if len(events) < pageSize {
			break
		}

		// Advance cursor strictly older than the oldest result on this
		// page. nostrdb returns newest-first per the descending order
		// we set in TextSearch.
		oldestTs := events[0].CreatedAt
		for _, e := range events {
			if e.CreatedAt < oldestTs {
				oldestTs = e.CreatedAt
			}
		}
		next := time.Unix(oldestTs-1, 0)
		if f.Since != nil && !next.After(*f.Since) {
			break
		}
		cursor = &next
	}
	return acc, nil
}

// hashFilters creates a deterministic hash of filter contents
func hashFilters(filters []nostr.Filter) string {
	// Serialize filters to JSON for comparison
	data, err := json.Marshal(filters)
	if err != nil {
		return "" // If serialization fails, treat as different
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
