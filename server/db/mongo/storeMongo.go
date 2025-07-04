package mongo

import (
	"context"
	"fmt"

	"github.com/0ceanslim/grain/server/db/mongo/eventStore"
	"github.com/0ceanslim/grain/server/handlers/response"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"

	"github.com/0ceanslim/grain/config"
)

// StoreMongoEvent processes and stores an event based on its kind
func StoreMongoEvent(ctx context.Context, evt nostr.Event, client nostr.ClientInterface) {
	collection := GetCollection(evt.Kind)
	dbName := config.GetConfig().MongoDB.Database

	// Get event category for logging
	category := utils.DetermineEventCategory(evt.Kind)

	log.MongoStore().Debug("Processing event for storage",
		"event_id", evt.ID,
		"kind", evt.Kind,
		"category", category,
		"pubkey", evt.PubKey)

	var err error
	switch {
	case evt.Kind == 2:
		log.MongoStore().Debug("Handling deprecated event", "event_id", evt.ID)
		err = eventStore.Deprecated(ctx, evt, client)

	case evt.Kind == 5:
		log.MongoStore().Debug("Handling deletion event", "event_id", evt.ID)
		err = eventStore.Delete(ctx, evt, GetClient(), dbName, client)

	case (evt.Kind >= 1000 && evt.Kind < 10000) ||
		(evt.Kind >= 4 && evt.Kind < 45) || evt.Kind == 1:
		log.MongoStore().Debug("Handling regular event",
			"event_id", evt.ID,
			"kind", evt.Kind)
		err = eventStore.Regular(ctx, evt, collection, client)

	case (evt.Kind >= 10000 && evt.Kind < 20000) ||
		evt.Kind == 0 || evt.Kind == 3:
		log.MongoStore().Debug("Handling replaceable event",
			"event_id", evt.ID,
			"kind", evt.Kind)
		err = eventStore.Replaceable(ctx, evt, collection, client)

	case evt.Kind >= 20000 && evt.Kind < 30000:
		log.MongoStore().Info("Ephemeral event received and ignored",
			"event_id", evt.ID,
			"kind", evt.Kind)

	case evt.Kind >= 30000 && evt.Kind < 40000:
		log.MongoStore().Debug("Handling addressable event",
			"event_id", evt.ID,
			"kind", evt.Kind)
		err = eventStore.Addressable(ctx, evt, collection, client)

	default:
		log.MongoStore().Warn("Handling unknown event kind",
			"event_id", evt.ID,
			"kind", evt.Kind)
		err = eventStore.Unknown(ctx, evt, collection, client)
	}

	if err != nil {
		log.MongoStore().Error("Failed to store event",
			"event_id", evt.ID,
			"kind", evt.Kind,
			"category", category,
			"error", err)
		response.SendOK(client, evt.ID, false, fmt.Sprintf("error: %v", err))
		return
	}

	log.MongoStore().Info("Event stored successfully",
		"event_id", evt.ID,
		"kind", evt.Kind,
		"category", category)
	response.SendOK(client, evt.ID, true, "")
}
