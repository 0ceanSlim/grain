package tests

import (
	"grain/relay/utils"
	"testing"
)

func TestSizeLimiterGlobalMaxSize(t *testing.T) {
	sizeLimiter := utils.NewSizeLimiter(1024) // Set global max size to 1024 bytes

	// Test that an event within the global max size is allowed
	if allowed, _ := sizeLimiter.AllowSize(0, 512); !allowed {
		t.Error("Event within global max size should be allowed")
	}

	// Test that an event exceeding the global max size is not allowed
	if allowed, msg := sizeLimiter.AllowSize(0, 2048); allowed {
		t.Error("Event exceeding global max size should not be allowed")
	} else {
		expectedMsg := "Global event size limit exceeded"
		if msg != expectedMsg {
			t.Errorf("Expected message: %s, got: %s", expectedMsg, msg)
		}
	}
}

func TestSizeLimiterKindSpecificSize(t *testing.T) {
	sizeLimiter := utils.NewSizeLimiter(1024) // Set global max size to 1024 bytes
	sizeLimiter.AddKindSizeLimit(1, 512)      // Set max size for kind 1 to 512 bytes

	// Test that an event within the kind-specific max size is allowed
	if allowed, _ := sizeLimiter.AllowSize(1, 256); !allowed {
		t.Error("Event within kind-specific max size should be allowed")
	}

	// Test that an event exceeding the kind-specific max size is not allowed
	if allowed, msg := sizeLimiter.AllowSize(1, 1024); allowed {
		t.Error("Event exceeding kind-specific max size should not be allowed")
	} else {
		expectedMsg := "Event size limit exceeded for kind"
		if msg != expectedMsg {
			t.Errorf("Expected message: %s, got: %s", expectedMsg, msg)
		}
	}

	// Test that an event exceeding the global max size is not allowed even if within the kind-specific max size
	if allowed, msg := sizeLimiter.AllowSize(1, 2048); allowed {
		t.Error("Event exceeding global max size should not be allowed even if within kind-specific max size")
	} else {
		expectedMsg := "Global event size limit exceeded"
		if msg != expectedMsg {
			t.Errorf("Expected message: %s, got: %s", expectedMsg, msg)
		}
	}
}

func TestSizeLimiterNoKindSpecificLimit(t *testing.T) {
	sizeLimiter := utils.NewSizeLimiter(1024) // Set global max size to 1024 bytes

	// Test that an event for a kind without a specific limit is governed by the global limit
	if allowed, _ := sizeLimiter.AllowSize(2, 512); !allowed {
		t.Error("Event within global max size should be allowed for kinds without specific limit")
	}

	// Test that an event exceeding the global max size is not allowed for kinds without a specific limit
	if allowed, msg := sizeLimiter.AllowSize(2, 2048); allowed {
		t.Error("Event exceeding global max size should not be allowed for kinds without specific limit")
	} else {
		expectedMsg := "Global event size limit exceeded"
		if msg != expectedMsg {
			t.Errorf("Expected message: %s, got: %s", expectedMsg, msg)
		}
	}
}
