package mongo

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/0ceanslim/grain/server/db/mongo/eventStore"
	"github.com/0ceanslim/grain/server/handlers/response"
	relay "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"

	"github.com/0ceanslim/grain/config"
)

var storeLog *slog.Logger

func init() {
	storeLog = utils.GetLogger("mongo-store")
}

// StoreMongoEvent processes and stores an event based on its kind
func StoreMongoEvent(ctx context.Context, evt relay.Event, client relay.ClientInterface) {
	collection := GetCollection(evt.Kind)
	dbName := config.GetConfig().MongoDB.Database
	
	// Get event category for logging
	category := utils.DetermineEventCategory(evt.Kind)
	
	storeLog.Debug("Processing event for storage", 
		"event_id", evt.ID, 
		"kind", evt.Kind,
		"category", category,
		"pubkey", evt.PubKey)

	var err error
	switch {
	case evt.Kind == 2:
		storeLog.Debug("Handling deprecated event", "event_id", evt.ID)
		err = eventStore.Deprecated(ctx, evt, client)
		
	case evt.Kind == 5:
		storeLog.Debug("Handling deletion event", "event_id", evt.ID)
		err = eventStore.Delete(ctx, evt, GetClient(), dbName, client)
		
	case (evt.Kind >= 1000 && evt.Kind < 10000) ||
		(evt.Kind >= 4 && evt.Kind < 45) || evt.Kind == 1:
		storeLog.Debug("Handling regular event", 
			"event_id", evt.ID, 
			"kind", evt.Kind)
		err = eventStore.Regular(ctx, evt, collection, client)
		
	case (evt.Kind >= 10000 && evt.Kind < 20000) ||
		evt.Kind == 0 || evt.Kind == 3:
		storeLog.Debug("Handling replaceable event", 
			"event_id", evt.ID, 
			"kind", evt.Kind)
		err = eventStore.Replaceable(ctx, evt, collection, client)
		
	case evt.Kind >= 20000 && evt.Kind < 30000:
		storeLog.Info("Ephemeral event received and ignored", 
			"event_id", evt.ID, 
			"kind", evt.Kind)
		
	case evt.Kind >= 30000 && evt.Kind < 40000:
		storeLog.Debug("Handling addressable event", 
			"event_id", evt.ID, 
			"kind", evt.Kind)
		err = eventStore.Addressable(ctx, evt, collection, client)
		
	default:
		storeLog.Warn("Handling unknown event kind", 
			"event_id", evt.ID, 
			"kind", evt.Kind)
		err = eventStore.Unknown(ctx, evt, collection, client)
	}

	if err != nil {
		storeLog.Error("Failed to store event", 
			"event_id", evt.ID, 
			"kind", evt.Kind, 
			"category", category, 
			"error", err)
		response.SendOK(client, evt.ID, false, fmt.Sprintf("error: %v", err))
		return
	}

	storeLog.Info("Event stored successfully", 
		"event_id", evt.ID, 
		"kind", evt.Kind, 
		"category", category)
	response.SendOK(client, evt.ID, true, "")
}