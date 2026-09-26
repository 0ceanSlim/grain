package handlers

import (
	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/db/nostrdb"
	"github.com/0ceanslim/grain/server/handlers/response"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// HandleCount processes a NIP-45 "COUNT" message. The wire format
// mirrors REQ — `["COUNT", <sub_id>, <filter1>, ...]` — and the
// response is `["COUNT", <sub_id>, {"count": N}]` (with an optional
// `approximate: true` field).
//
// Filters go through the same strict parseFilters as REQ: the old
// duplicated lenient parsing is how a malformed field silently widened
// both into an unconstrained query.
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

	filters, err := parseFilters(message[2:])
	if err != nil {
		log.Req().Info("Rejected malformed COUNT filter", "sub_id", subID, "error", err)
		response.SendClosed(client, subID, "invalid: "+err.Error())
		return
	}

	for i := range filters {
		// Filter `limit` is intentionally ignored for COUNT — NIP-45
		// asks for total matches, not a paginated subset.
		filters[i].Limit = nil

		// NIP-17 DM privacy (#73): COUNT on protected kinds leaks message-
		// volume metadata. If a filter explicitly counts gift wraps, require
		// AUTH and constrain the count to the AUTHed pubkey's own p-tag, so a
		// caller can only count their own inbox — never someone else's.
		if FilterRequestsProtectedKind(filters[i]) {
			authed := GetAuthedPubkey(client)
			if authed == "" {
				log.Req().Info("COUNT for protected kind requires auth", "sub_id", subID)
				response.SendClosed(client, subID, "auth-required: authentication is required to count these events")
				return
			}
			filters[i].Tags["p"] = []string{authed}
		}
	}

	sendPrefixMatchNotice(client, filters)

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
