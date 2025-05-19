package handlers

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/db/mongo"
	"github.com/0ceanslim/grain/server/utils/userSync"

	"github.com/0ceanslim/grain/server/handlers/response"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/validation"

	relay "github.com/0ceanslim/grain/server/types"
)

// Package-level logger
var eventLog *slog.Logger

func init() {
	eventLog = utils.GetLogger("event-handler")
}

// HandleEvent processes an "EVENT" message
func HandleEvent(client relay.ClientInterface, message []interface{}) {
	if len(message) != 2 {
		eventLog.Error("Invalid EVENT message format")
		response.SendNotice(client, "", "Invalid EVENT message format")
		return
	}

	eventData, ok := message[1].(map[string]interface{})
	if !ok {
		eventLog.Error("Invalid event data format")
		response.SendNotice(client, "", "Invalid event data format")
		return
	}

	eventBytes, err := json.Marshal(eventData)
	if err != nil {
		eventLog.Error("Error marshaling event data", "error", err)
		response.SendNotice(client, "", "Error marshaling event data")
		return
	}

	var evt relay.Event
	err = json.Unmarshal(eventBytes, &evt)
	if err != nil {
		eventLog.Error("Error unmarshaling event data", "error", err)
		response.SendNotice(client, "", "Error unmarshaling event data")
		return
	}

	// Load config
	cfg := config.GetConfig()
	if cfg == nil {
		eventLog.Error("Failed to get server configuration")
		response.SendOK(client, evt.ID, false, "error: internal server error")
		return
	}

	// Validate event timestamps
	if !validation.ValidateEventTimestamp(evt, cfg) {
		eventLog.Warn("Invalid timestamp for event", "event_id", evt.ID)
		response.SendOK(client, evt.ID, false, "invalid: event created_at timestamp is out of allowed range")
		return
	}

	// Signature check
	if !validation.CheckSignature(evt) {
		eventLog.Error("Signature verification failed for event", "event_id", evt.ID)
		response.SendOK(client, evt.ID, false, "invalid: signature verification failed")
		return
	}

	eventSize := len(eventBytes)

	// Blacklist/Whitelist check
	result := validation.CheckBlacklistAndWhitelist(evt)
	if !result.Valid {
		eventLog.Info("Event rejected by blacklist/whitelist", "event_id", evt.ID)
		response.SendOK(client, evt.ID, false, result.Message)
		return
	}

	// Rate and size limit checks
	result = validation.CheckRateAndSizeLimits(evt, eventSize)
	if !result.Valid {
		eventLog.Info("Event rejected by rate/size limits", "event_id", evt.ID)
		response.SendOK(client, evt.ID, false, result.Message)
		return
	}

	// Duplicate event check
	isDuplicate, err := mongo.CheckDuplicateEvent(context.TODO(), evt)
	if err != nil {
		log.Printf("[ERROR] Error checking for duplicate event: ID=%s, Error=%v", evt.ID, err)
		response.SendOK(client, evt.ID, false, "error: internal server error during duplicate check")
		return
	}
	if isDuplicate {
		log.Printf("[INFO] Duplicate event detected: ID=%s", evt.ID)
		response.SendOK(client, evt.ID, false, "blocked: the database already contains this event")
		return
	}

	// Trigger Negentropy sync
	go userSync.UserSyncCheck(evt, cfg)

	// Store event in MongoDB
	mongo.StoreMongoEvent(context.TODO(), evt, client)
	log.Printf("[INFO] Event stored successfully: ID=%s", evt.ID)

	// Send to backup relay
	if cfg.BackupRelay.Enabled {
		go func() {
			err := utils.SendToBackupRelay(cfg.BackupRelay.URL, evt)
			if err != nil {
				log.Printf("[ERROR] Failed to send event %s to backup relay: %v", evt.ID, err)
			} else {
				log.Printf("[INFO] Event %s successfully sent to backup relay", evt.ID)
			}
		}()
	}

	log.Printf("[INFO] Event processing completed: ID=%s", evt.ID)
}