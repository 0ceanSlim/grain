package mongo

import (
	"context"
	"fmt"

	"github.com/0ceanslim/grain/server/db/mongo/kinds"
	"github.com/0ceanslim/grain/server/handlers/response"
	relay "github.com/0ceanslim/grain/server/types"

	"github.com/0ceanslim/grain/config"
)

// StoreMongoEvent processes and stores an event based on its kind
func StoreMongoEvent(ctx context.Context, evt relay.Event, client relay.ClientInterface) {
	collection := GetCollection(evt.Kind)
	dbName := config.GetConfig().MongoDB.Database // ✅ Get database name from config

	var err error
	switch {
	case evt.Kind == 2:
		err = kinds.HandleDeprecatedKind(ctx, evt, client)
	case evt.Kind == 5:
		err = kinds.HandleDeleteKind(ctx, evt, GetClient(), dbName, client) // ✅ Pass dbName
	case (evt.Kind >= 1000 && evt.Kind < 10000) ||
		(evt.Kind >= 4 && evt.Kind < 45) || evt.Kind == 1:
		err = kinds.HandleRegularKind(ctx, evt, collection, client)
	case (evt.Kind >= 10000 && evt.Kind < 20000) ||
		evt.Kind == 0 || evt.Kind == 3:
		err = kinds.HandleReplaceableKind(ctx, evt, collection, client)
	case evt.Kind >= 20000 && evt.Kind < 30000:
		fmt.Println("Ephemeral event received and ignored:", evt.ID)
	case evt.Kind >= 30000 && evt.Kind < 40000:
		err = kinds.HandleAddressableKind(ctx, evt, collection, client)
	default:
		err = kinds.HandleUnknownKind(ctx, evt, collection, client)
	}

	if err != nil {
		response.SendOK(client, evt.ID, false, fmt.Sprintf("error: %v", err))
		return
	}

	response.SendOK(client, evt.ID, true, "")
}
