package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"grain/config"
	"grain/server/db/mongo"

	"grain/server/handlers/response"
	"grain/server/utils"

	nostr "grain/server/types"

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

	var evt nostr.Event
	err = json.Unmarshal(eventBytes, &evt)
	if err != nil {
		fmt.Println("Error unmarshaling event data:", err)
		response.SendNotice(ws, "", "Error unmarshaling event data")
		return
	}

	// Signature check moved here
	if !utils.CheckSignature(evt) {
		response.SendOK(ws, evt.ID, false, "invalid: signature verification failed")
		return
	}

	eventSize := len(eventBytes) // Calculate event size

	if !handleBlacklistAndWhitelist(ws, evt) {
		return
	}

	if !handleRateAndSizeLimits(ws, evt, eventSize) {
		return
	}

	// This is where I'll handle storage for multiple database types in the future
	mongo.StoreMongoEvent(context.TODO(), evt, ws)

	fmt.Println("Event processed:", evt.ID)

}

func handleBlacklistAndWhitelist(ws *websocket.Conn, evt nostr.Event) bool {
	if config.GetConfig().DomainWhitelist.Enabled {
		domains := config.GetConfig().DomainWhitelist.Domains
		pubkeys, err := utils.FetchPubkeysFromDomains(domains)
		if err != nil {
			fmt.Println("Error fetching pubkeys from domains:", err)
			response.SendNotice(ws, "", "Error fetching pubkeys from domains")
			return false
		}
		for _, pubkey := range pubkeys {
			config.GetConfig().PubkeyWhitelist.Pubkeys = append(config.GetConfig().PubkeyWhitelist.Pubkeys, pubkey)
		}
	}

	if blacklisted, msg := config.CheckBlacklist(evt.PubKey, evt.Content); blacklisted {
		response.SendOK(ws, evt.ID, false, msg)
		return false
	}

	if config.GetConfig().KindWhitelist.Enabled && !config.IsKindWhitelisted(evt.Kind) {
		response.SendOK(ws, evt.ID, false, "not allowed: event kind is not whitelisted")
		return false
	}

	if config.GetConfig().PubkeyWhitelist.Enabled && !config.IsPubKeyWhitelisted(evt.PubKey) {
		response.SendOK(ws, evt.ID, false, "not allowed: pubkey or npub is not whitelisted")
		return false
	}

	return true
}

func handleRateAndSizeLimits(ws *websocket.Conn, evt nostr.Event, eventSize int) bool {
	rateLimiter := config.GetRateLimiter()
	sizeLimiter := config.GetSizeLimiter()
	category := determineCategory(evt.Kind)

	if allowed, msg := rateLimiter.AllowEvent(evt.Kind, category); !allowed {
		response.SendOK(ws, evt.ID, false, msg)
		return false
	}

	if allowed, msg := sizeLimiter.AllowSize(evt.Kind, eventSize); !allowed {
		response.SendOK(ws, evt.ID, false, msg)
		return false
	}

	return true
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
