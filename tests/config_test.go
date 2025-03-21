package tests

import (
	"testing"

	"github.com/0ceanslim/grain/config"
)

func TestConfigValidity(t *testing.T) {
	config, err := config.LoadConfig("../config.yml")
	if err != nil {
		t.Fatalf("Error loading config: %v", err)
	}

	// Check MongoDB settings
	if config.MongoDB.URI == "" {
		t.Error("MongoDB URI is required")
	}
	if config.MongoDB.Database == "" {
		t.Error("MongoDB database name is required")
	}

	// Check Server settings
	if config.Server.Port == "" {
		t.Error("Server port is required")
	}

	// Check Rate Limit settings
	if config.RateLimit.WsLimit == 0 {
		t.Error("WebSocket limit is required")
	}
	if config.RateLimit.WsBurst == 0 {
		t.Error("WebSocket burst is required")
	}
	if config.RateLimit.EventLimit == 0 {
		t.Error("Event limit is required")
	}
	if config.RateLimit.EventBurst == 0 {
		t.Error("Event burst is required")
	}
	if config.RateLimit.ReqLimit == 0 {
		t.Error("REQ limit is required")
	}
	if config.RateLimit.ReqBurst == 0 {
		t.Error("REQ burst is required")
	}
	if config.RateLimit.MaxEventSize == 0 {
		t.Error("Global maximum event size is required")
	}

	// Check Category Limits
	if len(config.RateLimit.CategoryLimits) == 0 {
		t.Log("Warning: No category limits set")
	}

	// Check Kind Limits
	if len(config.RateLimit.KindLimits) == 0 {
		t.Log("Warning: No kind limits set")
	}

	// Validate individual category limits
	for category, limits := range config.RateLimit.CategoryLimits {
		if limits.Limit == 0 {
			t.Errorf("Limit is required for category: %s", category)
		}
		if limits.Burst == 0 {
			t.Errorf("Burst is required for category: %s", category)
		}
	}

	// Validate individual kind limits
	for _, kindLimit := range config.RateLimit.KindLimits {
		if kindLimit.Limit == 0 {
			t.Errorf("Limit is required for kind: %d", kindLimit.Kind)
		}
		if kindLimit.Burst == 0 {
			t.Errorf("Burst is required for kind: %d", kindLimit.Kind)
		}
	}

	// Validate kind size limits
	for _, kindSizeLimit := range config.RateLimit.KindSizeLimits {
		if kindSizeLimit.MaxSize == 0 {
			t.Errorf("Maximum size is required for kind: %d", kindSizeLimit.Kind)
		}
	}
}
