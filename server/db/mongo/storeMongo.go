package mongo

import (
	"context"
	"fmt"

	"github.com/0ceanslim/grain/server/db/mongo/kinds"
	"github.com/0ceanslim/grain/server/handlers/response"
	relay "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"

	"github.com/0ceanslim/grain/config"
)

// StoreMongoEvent processes and stores an event based on its kind
func StoreMongoEvent(ctx context.Context, evt relay.Event, client relay.ClientInterface) {
	collection := GetCollection(evt.Kind)
	dbName := config.GetConfig().MongoDB.Database
	
	// Get event category for logging
	category := utils.DetermineEventCategory(evt.Kind)
	
	mongoLog.Debug("Processing event for storage", 
		"event_id", evt.ID, 
		"kind", evt.Kind,
		"category", category,
		"pubkey", evt.PubKey)

	var err error
	switch {
	case evt.Kind == 2:
		mongoLog.Debug("Handling deprecated event", "event_id", evt.ID)
		err = kinds.HandleDeprecatedKind(ctx, evt, client)
		
	case evt.Kind == 5:
		mongoLog.Debug("Handling deletion event", "event_id", evt.ID)
		err = kinds.HandleDeleteKind(ctx, evt, GetClient(), dbName, client)
		
	case (evt.Kind >= 1000 && evt.Kind < 10000) ||
		(evt.Kind >= 4 && evt.Kind < 45) || evt.Kind == 1:
		mongoLog.Debug("Handling regular event", 
			"event_id", evt.ID, 
			"kind", evt.Kind)
		err = kinds.HandleRegularKind(ctx, evt, collection, client)
		
	case (evt.Kind >= 10000 && evt.Kind < 20000) ||
		evt.Kind == 0 || evt.Kind == 3:
		mongoLog.Debug("Handling replaceable event", 
			"event_id", evt.ID, 
			"kind", evt.Kind)
		err = kinds.HandleReplaceableKind(ctx, evt, collection, client)
		
	case evt.Kind >= 20000 && evt.Kind < 30000:
		mongoLog.Info("Ephemeral event received and ignored", 
			"event_id", evt.ID, 
			"kind", evt.Kind)
		
	case evt.Kind >= 30000 && evt.Kind < 40000:
		mongoLog.Debug("Handling addressable event", 
			"event_id", evt.ID, 
			"kind", evt.Kind)
		err = kinds.HandleAddressableKind(ctx, evt, collection, client)
		
	default:
		mongoLog.Warn("Handling unknown event kind", 
			"event_id", evt.ID, 
			"kind", evt.Kind)
		err = kinds.HandleUnknownKind(ctx, evt, collection, client)
	}

	if err != nil {
		mongoLog.Error("Failed to store event", 
			"event_id", evt.ID, 
			"kind", evt.Kind, 
			"category", category, 
			"error", err)
		response.SendOK(client, evt.ID, false, fmt.Sprintf("error: %v", err))
		return
	}

	mongoLog.Info("Event stored successfully", 
		"event_id", evt.ID, 
		"kind", evt.Kind, 
		"category", category)
	response.SendOK(client, evt.ID, true, "")
}