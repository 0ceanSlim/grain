package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"grain/config"
	"grain/server/db/mongo"
	"time"

	"grain/server/handlers/response"
	"grain/server/utils"

	nostr "grain/server/types"

	"golang.org/x/net/websocket"
)

func HandleEvent(ws *websocket.Conn, message []interface{}) {
	if len(message) != 2 {
		fmt.Println("Invalid EVENT message format")
		response.SendNotice(ws, "", "Invalid EVENT message format")
		return
	}

	eventData, ok := message[1].(map[string]interface{})
	if !ok {
		fmt.Println("Invalid event data format")
		response.SendNotice(ws, "", "Invalid event data format")
		return
	}
	eventBytes, err := json.Marshal(eventData)
	if err != nil {
		fmt.Println("Error marshaling event data:", err)
		response.SendNotice(ws, "", "Error marshaling event data")
		return
	}

	var evt nostr.Event
	err = json.Unmarshal(eventBytes, &evt)
	if err != nil {
		fmt.Println("Error unmarshaling event data:", err)
		response.SendNotice(ws, "", "Error unmarshaling event data")
		return
	}

	// Validate event timestamps
	if !validateEventTimestamp(evt) {
		response.SendOK(ws, evt.ID, false, "invalid: event created_at timestamp is out of allowed range")
		return
	}

	// Signature check moved here
	if !utils.CheckSignature(evt) {
		response.SendOK(ws, evt.ID, false, "invalid: signature verification failed")
		return
	}

	eventSize := len(eventBytes)

	if !handleBlacklistAndWhitelist(ws, evt) {
		return
	}

	if !handleRateAndSizeLimits(ws, evt, eventSize) {
		return
	}

	// Store the event in MongoDB or other storage
	mongo.StoreMongoEvent(context.TODO(), evt, ws)
	fmt.Println("Event processed:", evt.ID)
}

// Validate event timestamps against the configured min and max values
func validateEventTimestamp(evt nostr.Event) bool {
	cfg := config.GetConfig()
	if cfg == nil {
		fmt.Println("Server configuration is not loaded")
		return false
	}

	// Adjust event time constraints in the configuration
	utils.AdjustEventTimeConstraints(cfg)

	// Use current time for max and a fixed date for min if not specified
	now := time.Now().Unix()
	minCreatedAt := cfg.EventTimeConstraints.MinCreatedAt
	if minCreatedAt == 0 {
		// Use January 1, 2020, as the default minimum timestamp
		minCreatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	}

	maxCreatedAt := cfg.EventTimeConstraints.MaxCreatedAt
	if maxCreatedAt == 0 {
		// Default to the current time if not set
		maxCreatedAt = now
	}

	// Check if the event's created_at timestamp falls within the allowed range
	if evt.CreatedAt < minCreatedAt || evt.CreatedAt > maxCreatedAt {
		fmt.Printf("Event %s created_at timestamp %d is out of range [%d, %d]\n", evt.ID, evt.CreatedAt, minCreatedAt, maxCreatedAt)
		return false
	}

	return true
}

func handleBlacklistAndWhitelist(ws *websocket.Conn, evt nostr.Event) bool {
    // Use the updated CheckBlacklist function
    if blacklisted, msg := config.CheckBlacklist(evt.PubKey, evt.Content); blacklisted {
        response.SendOK(ws, evt.ID, false, msg)
        return false
    }

    // Check the whitelist using CheckWhitelist function
    isWhitelisted, msg := config.CheckWhitelist(evt)
    if !isWhitelisted {
        response.SendOK(ws, evt.ID, false, msg)
        return false
    }

    return true
}

func handleRateAndSizeLimits(ws *websocket.Conn, evt nostr.Event, eventSize int) bool {
	rateLimiter := config.GetRateLimiter()
	sizeLimiter := config.GetSizeLimiter()
	category := determineCategory(evt.Kind)

	if allowed, msg := rateLimiter.AllowEvent(evt.Kind, category); !allowed {
		response.SendOK(ws, evt.ID, false, msg)
		return false
	}

	if allowed, msg := sizeLimiter.AllowSize(evt.Kind, eventSize); !allowed {
		response.SendOK(ws, evt.ID, false, msg)
		return false
	}

	return true
}

func determineCategory(kind int) string {
	switch {
	case kind == 0, kind == 3, kind >= 10000 && kind < 20000:
		return "replaceable"
	case kind == 1, kind >= 4 && kind < 45, kind >= 1000 && kind < 10000:
		return "regular"
	case kind == 2:
		return "deprecated"
	case kind >= 20000 && kind < 30000:
		return "ephemeral"
	case kind >= 30000 && kind < 40000:
		return "parameterized_replaceable"
	default:
		return "unknown"
	}
}
