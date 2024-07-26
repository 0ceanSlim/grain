package tests

import (
	"testing"

	"grain/relay/utils"

	"golang.org/x/time/rate"
)

func TestWebSocketRateLimit(t *testing.T) {
	rateLimiter := utils.NewRateLimiter(rate.Limit(1), 1, rate.Limit(100), 200, rate.Limit(100), 200)

	// First message should be allowed
	if allowed, _ := rateLimiter.AllowWs(); !allowed {
		t.Error("First WebSocket message should be allowed")
	}

	// Second message should be rate-limited
	if allowed, msg := rateLimiter.AllowWs(); allowed {
		t.Error("Second WebSocket message should be rate-limited")
	} else {
		expectedMsg := "WebSocket message rate limit exceeded"
		if msg != expectedMsg {
			t.Errorf("Expected message: %s, got: %s", expectedMsg, msg)
		}
	}
}

func TestEventRateLimit(t *testing.T) {
	rateLimiter := utils.NewRateLimiter(rate.Limit(100), 200, rate.Limit(1), 1, rate.Limit(100), 200)
	rateLimiter.AddKindLimit(1, rate.Limit(1), 1)
	rateLimiter.AddCategoryLimit("regular", rate.Limit(1), 1)

	// First event should be allowed
	if allowed, _ := rateLimiter.AllowEvent(1, "regular"); !allowed {
		t.Error("First event should be allowed")
	}

	// Second event should be rate-limited
	if allowed, msg := rateLimiter.AllowEvent(1, "regular"); allowed {
		t.Error("Second event should be rate-limited")
	} else {
		expectedMsg := "Global event rate limit exceeded"
		if msg != expectedMsg {
			t.Errorf("Expected message: %s, got: %s", expectedMsg, msg)
		}
	}
}

func TestReqRateLimit(t *testing.T) {
	rateLimiter := utils.NewRateLimiter(rate.Limit(100), 200, rate.Limit(100), 200, rate.Limit(1), 1)

	// First REQ should be allowed
	if allowed, _ := rateLimiter.AllowReq(); !allowed {
		t.Error("First REQ message should be allowed")
	}

	// Second REQ should be rate-limited
	if allowed, msg := rateLimiter.AllowReq(); allowed {
		t.Error("Second REQ message should be rate-limited")
	} else {
		expectedMsg := "REQ rate limit exceeded"
		if msg != expectedMsg {
			t.Errorf("Expected message: %s, got: %s", expectedMsg, msg)
		}
	}
}

func TestKindRateLimit(t *testing.T) {
	rateLimiter := utils.NewRateLimiter(rate.Limit(100), 200, rate.Limit(100), 200, rate.Limit(100), 200)
	rateLimiter.AddKindLimit(1, rate.Limit(1), 1)

	// First event of kind 1 should be allowed
	if allowed, _ := rateLimiter.AllowEvent(1, "regular"); !allowed {
		t.Error("First event of kind 1 should be allowed")
	}

	// Second event of kind 1 should be rate-limited
	if allowed, msg := rateLimiter.AllowEvent(1, "regular"); allowed {
		t.Error("Second event of kind 1 should be rate-limited")
	} else {
		expectedMsg := "Rate limit exceeded for kind: 1"
		if msg != expectedMsg {
			t.Errorf("Expected message: %s, got: %s", expectedMsg, msg)
		}
	}
}

func TestCategoryRateLimit(t *testing.T) {
	rateLimiter := utils.NewRateLimiter(rate.Limit(100), 200, rate.Limit(100), 200, rate.Limit(100), 200)
	rateLimiter.AddCategoryLimit("regular", rate.Limit(1), 1)

	// First event in category "regular" should be allowed
	if allowed, _ := rateLimiter.AllowEvent(1, "regular"); !allowed {
		t.Error("First event in category 'regular' should be allowed")
	}

	// Second event in category "regular" should be rate-limited
	if allowed, msg := rateLimiter.AllowEvent(1, "regular"); allowed {
		t.Error("Second event in category 'regular' should be rate-limited")
	} else {
		expectedMsg := "Rate limit exceeded for category: regular"
		if msg != expectedMsg {
			t.Errorf("Expected message: %s, got: %s", expectedMsg, msg)
		}
	}
}
