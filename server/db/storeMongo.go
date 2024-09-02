package db

import (
	"context"
	"fmt"
	"grain/server/handlers/kinds"
	"grain/server/handlers/response"
	nostr "grain/server/types"

	"golang.org/x/net/websocket"
)

func StoreMongoEvent(ctx context.Context, evt nostr.Event, ws *websocket.Conn) {
	collection := GetCollection(evt.Kind)

	var err error
	switch {
	case evt.Kind == 0:
		err = kinds.HandleKind0(ctx, evt, collection, ws)
	case evt.Kind == 1:
		err = kinds.HandleKind1(ctx, evt, collection, ws)
	case evt.Kind == 2:
		err = kinds.HandleKind2(ctx, evt, ws)
	case evt.Kind == 3:
		err = kinds.HandleReplaceableKind(ctx, evt, collection, ws)
	case evt.Kind == 5:
		err = kinds.HandleKind5(ctx, evt, GetClient(), ws)
	case evt.Kind >= 4 && evt.Kind < 45:
		err = kinds.HandleRegularKind(ctx, evt, collection, ws)
	case evt.Kind >= 1000 && evt.Kind < 10000:
		err = kinds.HandleRegularKind(ctx, evt, collection, ws)
	case evt.Kind >= 10000 && evt.Kind < 20000:
		err = kinds.HandleReplaceableKind(ctx, evt, collection, ws)
	case evt.Kind >= 20000 && evt.Kind < 30000:
		fmt.Println("Ephemeral event received and ignored:", evt.ID)
	case evt.Kind >= 30000 && evt.Kind < 40000:
		err = kinds.HandleParameterizedReplaceableKind(ctx, evt, collection, ws)
	default:
		err = kinds.HandleUnknownKind(ctx, evt, collection, ws)
	}

	if err != nil {
		response.SendOK(ws, evt.ID, false, fmt.Sprintf("error: %v", err))
		return
	}

	response.SendOK(ws, evt.ID, true, "")
}
