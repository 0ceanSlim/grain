package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/db/nostrdb"
	"github.com/0ceanslim/grain/server/handlers/response"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
	"github.com/0ceanslim/grain/server/validation"
)

// OnEventStored is called after an event is successfully stored.
// Set by the server package to broadcast events to active subscribers.
var OnEventStored func(evt nostr.Event)

// HandleEvent processes an "EVENT" message
func HandleEvent(client nostr.ClientInterface, message []interface{}) {
	if len(message) != 2 {
		log.Event().Error("Invalid EVENT message format")
		response.SendNotice(client, "", "Invalid EVENT message format")
		return
	}

	eventData, ok := message[1].(map[string]interface{})
	if !ok {
		log.Event().Error("Invalid event data format")
		response.SendNotice(client, "", "Invalid event data format")
		return
	}

	eventBytes, err := json.Marshal(eventData)
	if err != nil {
		log.Event().Error("Error marshaling event data", "error", err)
		response.SendNotice(client, "", "Error marshaling event data")
		return
	}

	var evt nostr.Event
	err = json.Unmarshal(eventBytes, &evt)
	if err != nil {
		log.Event().Error("Error unmarshaling event data", "error", err)
		response.SendNotice(client, "", "Error unmarshaling event data")
		return
	}

	// Load config
	cfg := config.GetConfig()

	// Enforce NIP-42 authentication if required
	if cfg.Auth.Required {
		if !IsAuthenticated(client) {
			log.Event().Info("EVENT rejected: authentication required", "event_id", evt.ID)
			response.SendOK(client, evt.ID, false, "auth-required: authentication is required to use this relay")
			return
		}
	}

	// NIP-70: events with a `["-"]` tag are protected and may only be
	// accepted from an authenticated connection whose pubkey matches
	// the event author.
	if validation.IsProtectedEvent(evt) {
		authedPubkey := GetAuthedPubkey(client)
		if authedPubkey == "" {
			log.Event().Info("EVENT rejected: protected event requires authentication (NIP-70)",
				"event_id", evt.ID, "pubkey", evt.PubKey)
			response.SendOK(client, evt.ID, false, "auth-required: this event is protected and requires authentication")
			return
		}
		if authedPubkey != evt.PubKey {
			log.Event().Info("EVENT rejected: protected event author mismatch (NIP-70)",
				"event_id", evt.ID, "event_pubkey", evt.PubKey, "authed_pubkey", authedPubkey)
			response.SendOK(client, evt.ID, false, "restricted: protected events may only be published by their author")
			return
		}
	}

	if cfg == nil {
		log.Event().Error("Failed to get server configuration")
		response.SendOK(client, evt.ID, false, "error: internal server error")
		return
	}

	// Validate event timestamps
	if !validation.ValidateEventTimestamp(evt, cfg) {
		log.Event().Warn("Invalid timestamp for event", "event_id", evt.ID)
		response.SendOK(client, evt.ID, false, "invalid: event created_at timestamp is out of allowed range")
		return
	}

	// NIP-40: reject events whose expiration has already passed at ingest time.
	// Future expirations are stored normally and will be swept by the
	// expiration tracker (see server/db/nostrdb/expiration.go).
	if validation.IsExpired(evt, time.Now().Unix()) {
		log.Event().Info("EVENT rejected: expired (NIP-40)", "event_id", evt.ID)
		response.SendOK(client, evt.ID, false, "invalid: event is expired")
		return
	}

	// Signature check
	if !validation.CheckSignature(evt) {
		log.Event().Error("Signature verification failed", "event_id", evt.ID)
		response.SendOK(client, evt.ID, false, "invalid: signature verification failed")
		return
	}

	eventSize := len(eventBytes)

	// Blacklist/Whitelist check - uses validation methods that respect enabled state
	result := validation.CheckBlacklistAndWhitelistCached(evt)
	if !result.Valid {
		log.Event().Info("Event rejected by cached blacklist/whitelist",
			"event_id", evt.ID,
			"pubkey", evt.PubKey,
			"reason", result.Message)
		response.SendOK(client, evt.ID, false, result.Message)
		return
	}

	// Per-client rate and size limit checks
	result = validation.CheckRateAndSizeLimits(client, evt, eventSize)
	if !result.Valid {
		log.Event().Info("Event rejected by rate/size limits",
			"event_id", evt.ID,
			"kind", evt.Kind,
			"size", eventSize,
			"reason", result.Message)
		response.SendOK(client, evt.ID, false, result.Message)
		return
	}

	// Check database availability
	db := nostrdb.GetDB()
	if db == nil {
		log.Event().Error("Database not available", "event_id", evt.ID)
		response.SendOK(client, evt.ID, false, "error: database not available")
		return
	}

	// Duplicate event check
	isDuplicate, err := db.CheckDuplicateEvent(evt)
	if err != nil {
		log.Event().Error("Error checking for duplicate event",
			"event_id", evt.ID,
			"error", err)
		response.SendOK(client, evt.ID, false, "error: internal server error during duplicate check")
		return
	}

	if isDuplicate {
		log.Event().Info("Duplicate event detected", "event_id", evt.ID)
		response.SendOK(client, evt.ID, false, "duplicate: already have this event")
		response.SendNotice(client, evt.PubKey, fmt.Sprintf("event %s was rejected because the relay already stores it", evt.ID))
		return
	}

	// Store event in nostrdb
	var storeErr error
	if evt.Kind == 5 {
		storeErr = db.ProcessDeletion(context.TODO(), evt)
	} else {
		storeErr = db.StoreEvent(context.TODO(), evt)
	}

	if storeErr != nil {
		// Storage layer returns errors prefixed with NIP-01 OK reasons
		// (`blocked:`, `duplicate:`, `invalid:`) for normal client-facing
		// rejections. Pass those through verbatim and log at INFO — the
		// client is expected to handle them, not the operator. Anything
		// else is a real failure: log ERROR and wrap with `error:`.
		msg := storeErr.Error()
		if isClientFacingReject(msg) {
			log.Event().Info("Event rejected",
				"event_id", evt.ID,
				"kind", evt.Kind,
				"reason", msg)
			response.SendOK(client, evt.ID, false, msg)
			response.SendNotice(client, evt.PubKey, fmt.Sprintf("event %s was rejected: %s", evt.ID, msg))
			return
		}
		log.Event().Error("Failed to store event",
			"event_id", evt.ID,
			"kind", evt.Kind,
			"error", storeErr)
		response.SendOK(client, evt.ID, false, fmt.Sprintf("error: %v", storeErr))
		return
	}

	response.SendOK(client, evt.ID, true, "")
	log.Event().Info("Event stored successfully",
		"event_id", evt.ID,
		"kind", evt.Kind,
		"pubkey", evt.PubKey)

	// Broadcast to active subscribers
	if OnEventStored != nil {
		OnEventStored(evt)
	}

	// Send to backup relay(s). Fan out in parallel goroutines —
	// one slow upstream shouldn't block the others, and the
	// ingestion path has already completed (this is purely
	// best-effort forwarding).
	if cfg.BackupRelay.Enabled {
		for _, url := range cfg.BackupRelay.URLs {
			url := url // capture per-iteration for the goroutine
			go func() {
				if err := utils.SendToBackupRelay(url, evt); err != nil {
					log.Event().Error("Failed to send event to backup relay",
						"event_id", evt.ID,
						"relay_url", url,
						"error", err)
				} else {
					log.Event().Info("Event sent to backup relay",
						"event_id", evt.ID,
						"relay_url", url)
				}
			}()
		}
	}

	log.Event().Info("Event processing completed",
		"event_id", evt.ID,
		"kind", evt.Kind,
		"pubkey", evt.PubKey)
}

// isClientFacingReject reports whether a storage error message starts with a
// NIP-01 OK-machine-readable prefix that the client is expected to handle
// (`blocked:`, `duplicate:`, `invalid:`). These are normal client interactions
// — log at INFO, not ERROR.
func isClientFacingReject(msg string) bool {
	for _, p := range []string{"blocked:", "duplicate:", "invalid:"} {
		if strings.HasPrefix(msg, p) {
			return true
		}
	}
	return false
}
