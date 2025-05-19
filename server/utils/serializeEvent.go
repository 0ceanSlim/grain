package utils

import (
	"encoding/json"

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
		utilLog().Error("Failed to serialize event", 
			"event_id", evt.ID,
			"pubkey", evt.PubKey,
			"kind", evt.Kind,
			"error", err)
		return ""
	}
	
	// Only log at debug level for very important events or when troubleshooting
	if evt.Kind == 0 || evt.Kind == 3 {
		utilLog().Debug("Event serialized", 
			"event_id", evt.ID,
			"kind", evt.Kind,
			"size_bytes", len(jsonBytes))
	}
	
	return string(jsonBytes)
}