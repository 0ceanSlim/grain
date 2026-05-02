package handlers

import (
	"context"
	"encoding/json"
	"fmt"
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
		response.SendOK(client, evt.ID, false, "blocked: the database already contains this event")
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

	// Send to backup relay
	if cfg.BackupRelay.Enabled {
		go func() {
			err := utils.SendToBackupRelay(cfg.BackupRelay.URL, evt)
			if err != nil {
				log.Event().Error("Failed to send event to backup relay",
					"event_id", evt.ID,
					"relay_url", cfg.BackupRelay.URL,
					"error", err)
			} else {
				log.Event().Info("Event sent to backup relay",
					"event_id", evt.ID,
					"relay_url", cfg.BackupRelay.URL)
			}
		}()
	}

	log.Event().Info("Event processing completed",
		"event_id", evt.ID,
		"kind", evt.Kind,
		"pubkey", evt.PubKey)
}
