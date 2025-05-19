package utils

import (
	"encoding/json"
	"log"

	relay "github.com/0ceanslim/grain/server/types"
)

// SerializeEvent manually constructs the JSON string for event serialization according to NIP-01
func SerializeEvent(evt relay.Event) string {
	eventData := []interface{}{
		0,
		evt.PubKey,
		evt.CreatedAt,
		evt.Kind,
		evt.Tags,
		evt.Content,
	}
	jsonBytes, err := json.Marshal(eventData)
	if err != nil {
		log.Printf("Error serializing event: %v", err)
		return ""
	}
	return string(jsonBytes)
}
