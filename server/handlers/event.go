package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"grain/config"
	"grain/server/db"
	"grain/server/handlers/kinds"
	"grain/server/handlers/response"
	"grain/server/utils"

	relay "grain/server/types"

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

	var evt relay.Event
	err = json.Unmarshal(eventBytes, &evt)
	if err != nil {
		fmt.Println("Error unmarshaling event data:", err)
		response.SendNotice(ws, "", "Error unmarshaling event data")
		return
	}

	eventSize := len(eventBytes) // Calculate event size
	HandleKind(context.TODO(), evt, ws, eventSize)

	fmt.Println("Event processed:", evt.ID)
}

func HandleKind(ctx context.Context, evt relay.Event, ws *websocket.Conn, eventSize int) {
	if !utils.CheckSignature(evt) {
		response.SendOK(ws, evt.ID, false, "invalid: signature verification failed")
		return
	}

	collection := db.GetCollection(evt.Kind)
	rateLimiter := config.GetRateLimiter()
	sizeLimiter := config.GetSizeLimiter()

	// Check whitelist
	if !isWhitelisted(evt.PubKey) {
		response.SendOK(ws, evt.ID, false, "not allowed: pubkey is not whitelisted")
		return
	}

	category := determineCategory(evt.Kind)

	if allowed, msg := rateLimiter.AllowEvent(evt.Kind, category); !allowed {
		response.SendOK(ws, evt.ID, false, msg)
		return
	}

	if allowed, msg := sizeLimiter.AllowSize(evt.Kind, eventSize); !allowed {
		response.SendOK(ws, evt.ID, false, msg)
		return
	}

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
		err = kinds.HandleKind5(ctx, evt, db.GetClient(), ws)
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

// Helper function to check if a pubkey is whitelisted
func isWhitelisted(pubKey string) bool {
	cfg := config.GetConfig()
	if !cfg.Whitelist.Enabled {
		return true
	}
	for _, whitelistedKey := range cfg.Whitelist.Pubkeys {
		if pubKey == whitelistedKey {
			return true
		}
	}
	return false
}
