package utils

import (
	"encoding/json"
	"strings"

	relay "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
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
	
	// Use Go's standard JSON marshaling first
	jsonBytes, err := json.Marshal(eventData)
	if err != nil {
		log.Util().Error("Failed to serialize event", 
			"event_id", evt.ID,
			"pubkey", evt.PubKey,
			"kind", evt.Kind,
			"error", err)
		return ""
	}
	
	// Convert to NIP-01 compliant format
	jsonStr := string(jsonBytes)
	jsonStr = normalizeJSONForNIP01(jsonStr)
	
	// Only log at debug level for very important events or when troubleshooting
	if evt.Kind == 0 || evt.Kind == 3 {
		log.Util().Debug("Event serialized", 
			"event_id", evt.ID,
			"kind", evt.Kind,
			"size_bytes", len(jsonStr))
	}
	
	return jsonStr
}

// normalizeJSONForNIP01 converts Go's JSON output to NIP-01 compliant format
func normalizeJSONForNIP01(jsonStr string) string {
	// Go's json.Marshal escapes some characters that NIP-01 says should NOT be escaped
	// We need to unescape Unicode sequences like \u0026 back to their original form
	
	// Replace common Unicode escapes that Go adds but NIP-01 doesn't require
	replacements := map[string]string{
		"\\u0026": "&",  // Ampersand
		"\\u003c": "<",  // Less than
		"\\u003e": ">",  // Greater than
		"\\u003d": "=",  // Equals sign
		"\\u002b": "+",  // Plus sign
		"\\u0027": "'",  // Single quote (apostrophe)
		"\\u002f": "/",  // Forward slash
	}
	
	result := jsonStr
	for escaped, unescaped := range replacements {
		result = strings.ReplaceAll(result, escaped, unescaped)
	}
	
	return result
}