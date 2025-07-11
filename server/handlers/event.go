package handlers

import (
	"context"
	"encoding/json"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/db/mongo"
	"github.com/0ceanslim/grain/server/handlers/response"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
	"github.com/0ceanslim/grain/server/utils/userSync"
	"github.com/0ceanslim/grain/server/validation"
)

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

	// Rate and size limit checks
	result = validation.CheckRateAndSizeLimits(evt, eventSize)
	if !result.Valid {
		log.Event().Info("Event rejected by rate/size limits",
			"event_id", evt.ID,
			"kind", evt.Kind,
			"size", eventSize,
			"reason", result.Message)
		response.SendOK(client, evt.ID, false, result.Message)
		return
	}

	// Duplicate event check
	isDuplicate, err := mongo.CheckDuplicateEvent(context.TODO(), evt)
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

	// Trigger UserSyncCheck sync
	go userSync.UserSyncCheckCached(evt, cfg)

	// Store event in MongoDB
	mongo.StoreMongoEvent(context.TODO(), evt, client)
	log.Event().Info("Event stored successfully",
		"event_id", evt.ID,
		"kind", evt.Kind,
		"pubkey", evt.PubKey)

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
