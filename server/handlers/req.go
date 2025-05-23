package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/db/mongo"
	"github.com/0ceanslim/grain/server/handlers/response"
	relay "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
)

// Set the logging component for REQ handler
func reqLog() *slog.Logger {
	return utils.GetLogger("req-handler")
}


// HandleReq processes a new subscription request with proper subscription management
func HandleReq(client relay.ClientInterface, message []interface{}) {
	if len(message) < 3 {
		reqLog().Error("Invalid REQ message format")
		response.SendClosed(client, "", "invalid: invalid REQ message format")
		return
	}

	subscriptions := client.GetSubscriptions()

	subID, ok := message[1].(string)
	if !ok || len(subID) == 0 || len(subID) > 64 {
		reqLog().Error("Invalid subscription ID format or length", 
			"sub_id", subID, 
			"length", len(subID))
		response.SendClosed(client, "", "invalid: subscription ID must be between 1 and 64 characters long")
		return
	}

	// Add REQ rate limiting check
	rateLimiter := config.GetRateLimiter()
	if rateLimiter != nil {
		if allowed, msg := rateLimiter.AllowReq(); !allowed {
			reqLog().Warn("REQ rate limit exceeded", 
				"sub_id", subID,
				"reason", msg)
			response.SendClosed(client, subID, "rate-limited: "+msg)
			return
		}
	}

	// Parse and validate filters
	filters := make([]relay.Filter, len(message)-2)
	for i, filter := range message[2:] {
		filterData, ok := filter.(map[string]interface{})
		if !ok {
			reqLog().Error("Invalid filter format", 
				"sub_id", subID, 
				"filter_index", i)
			response.SendClosed(client, subID, "invalid: invalid filter format")
			return
		}

		var f relay.Filter
		f.IDs = utils.ToStringArray(filterData["ids"])
		f.Authors = utils.ToStringArray(filterData["authors"])
		f.Kinds = utils.ToIntArray(filterData["kinds"])
		f.Tags = utils.ToTagsMap(filterData["tags"])
		f.Since = utils.ToTime(filterData["since"])
		f.Until = utils.ToTime(filterData["until"])
		f.Limit = utils.ToInt(filterData["limit"])

		filters[i] = f
	}

	// Check if this is a duplicate subscription (same filters)
	if existingFilters, exists := subscriptions[subID]; exists {
		if areFiltersIdentical(existingFilters, filters) {
			reqLog().Debug("Duplicate subscription detected, ignoring", 
				"sub_id", subID,
				"filter_count", len(filters))
			// Still send EOSE for duplicate subscriptions to satisfy client expectations
			client.SendMessage([]interface{}{"EOSE", subID})
			return
		} else {
			reqLog().Info("Subscription updated with new filters", 
				"sub_id", subID,
				"old_filter_count", len(existingFilters),
				"new_filter_count", len(filters))
		}
	}

	// Remove oldest subscription if needed
	if len(subscriptions) >= config.GetConfig().Server.MaxSubscriptionsPerClient {
		var oldestSubID string
		for id := range subscriptions {
			if id != subID { // Don't remove the current subscription
				oldestSubID = id
				break
			}
		}
		if oldestSubID != "" {
			delete(subscriptions, oldestSubID)
			reqLog().Info("Dropped oldest subscription", 
				"old_sub_id", oldestSubID, 
				"current_count", len(subscriptions))
		}
	}

	// Add/update subscription - THIS IS CRUCIAL: subscription stays active after EOSE
	subscriptions[subID] = filters
	reqLog().Info("Subscription created/updated", 
		"sub_id", subID, 
		"filter_count", len(filters), 
		"total_subscriptions", len(subscriptions))

	// Query database for historical events
	dbName := config.GetConfig().MongoDB.Database
	queriedEvents, err := mongo.QueryEvents(filters, mongo.GetClient(), dbName)
	if err != nil {
		reqLog().Error("Error querying events", 
			"sub_id", subID, 
			"database", dbName, 
			"error", err)
		response.SendClosed(client, subID, "error: could not query events")
		return
	}

	// Send historical events to client
	for _, evt := range queriedEvents {
		client.SendMessage([]interface{}{"EVENT", subID, evt})
	}

	// Send EOSE message to indicate end of stored events
	client.SendMessage([]interface{}{"EOSE", subID})

	reqLog().Info("Subscription established", 
		"sub_id", subID, 
		"historical_events_sent", len(queriedEvents),
		"status", "active")

	// NOTE: Subscription remains ACTIVE after EOSE
	// It will be closed only when:
	// 1. Client sends CLOSE message
	// 2. Client disconnects
	// 3. New REQ with same subID (replaces this one)
	// 4. Subscription limit reached (oldest removed)
}

// areFiltersIdentical compares two filter slices to detect duplicates
func areFiltersIdentical(filters1, filters2 []relay.Filter) bool {
	if len(filters1) != len(filters2) {
		return false
	}

	// Simple approach: serialize both and compare hashes
	hash1 := hashFilters(filters1)
	hash2 := hashFilters(filters2)
	
	return hash1 == hash2
}

// hashFilters creates a deterministic hash of filter contents
func hashFilters(filters []relay.Filter) string {
	// Serialize filters to JSON for comparison
	data, err := json.Marshal(filters)
	if err != nil {
		return "" // If serialization fails, treat as different
	}
	
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}