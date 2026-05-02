package handlers

import (
	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/db/nostrdb"
	"github.com/0ceanslim/grain/server/handlers/response"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// HandleCount processes a NIP-45 "COUNT" message. The wire format
// mirrors REQ — `["COUNT", <sub_id>, <filter1>, ...]` — and the
// response is `["COUNT", <sub_id>, {"count": N}]` (with an optional
// `approximate: true` field).
//
// Filter parsing intentionally duplicates the REQ-side logic rather
// than sharing it: REQ also creates a long-lived subscription; COUNT
// is a one-shot read. The shared bits are small enough that a helper
// would obscure more than it saves.
func HandleCount(client nostr.ClientInterface, message []interface{}) {
	if len(message) < 3 {
		log.Req().Error("Invalid COUNT message format")
		response.SendClosed(client, "", "invalid: invalid COUNT message format")
		return
	}

	subID, ok := message[1].(string)

	cfg := config.GetConfig()
	if cfg.Auth.Required {
		if !IsAuthenticated(client) {
			log.Req().Info("COUNT rejected: authentication required", "sub_id", subID)
			response.SendClosed(client, subID, "auth-required: authentication is required to use this relay")
			return
		}
	}

	if !ok || len(subID) == 0 || len(subID) > 64 {
		log.Req().Error("Invalid COUNT subscription ID format or length",
			"sub_id", subID, "length", len(subID))
		response.SendClosed(client, "", "invalid: subscription ID must be between 1 and 64 characters long")
		return
	}

	// Reuse the per-client REQ rate limiter — COUNT is a query op and a
	// flood of COUNTs can be just as expensive as a flood of REQs.
	if allowed, msg := client.AllowReq(); !allowed {
		log.Req().Warn("COUNT rate limit exceeded", "sub_id", subID, "reason", msg)
		response.SendClosed(client, subID, "rate-limited: "+msg)
		return
	}

	filters := make([]nostr.Filter, len(message)-2)
	for i, filter := range message[2:] {
		filterData, ok := filter.(map[string]interface{})
		if !ok {
			log.Req().Error("Invalid COUNT filter format", "sub_id", subID, "filter_index", i)
			response.SendClosed(client, subID, "invalid: invalid filter format")
			return
		}

		var f nostr.Filter
		f.IDs = utils.ToStringArray(filterData["ids"])
		f.Authors = utils.ToStringArray(filterData["authors"])
		f.Kinds = utils.ToIntArray(filterData["kinds"])
		f.Since = utils.ToTime(filterData["since"])
		f.Until = utils.ToTime(filterData["until"])

		f.Tags = make(map[string][]string)
		for k, v := range filterData {
			if len(k) >= 2 && k[0] == '#' {
				tagName := k[1:]
				if vals := utils.ToStringArray(v); len(vals) > 0 {
					f.Tags[tagName] = vals
				}
			}
		}
		// Filter `limit` is intentionally ignored for COUNT — NIP-45
		// asks for total matches, not a paginated subset.

		filters[i] = f
	}

	db := nostrdb.GetDB()
	if db == nil {
		log.Req().Error("Database not available for COUNT", "sub_id", subID)
		response.SendClosed(client, subID, "error: database not available")
		return
	}

	count, approximate, err := db.CountFiltered(filters)
	if err != nil {
		log.Req().Error("COUNT query failed", "sub_id", subID, "error", err)
		response.SendClosed(client, subID, "error: could not count events")
		return
	}

	response.SendCount(client, subID, count, approximate)

	log.Req().Info("COUNT served",
		"sub_id", subID,
		"filter_count", len(filters),
		"count", count,
		"approximate", approximate)
}
