package config

import "testing"

func TestSizeLimiterAllowContentLength(t *testing.T) {
	sl := NewSizeLimiter(1000)

	// 0 (default) means no content-length limit.
	if ok, _ := sl.AllowContentLength(1_000_000); !ok {
		t.Error("a content-length limit of 0 should allow any length")
	}

	// A positive limit is enforced; the boundary value is allowed.
	sl.maxContentLength = 280
	if ok, _ := sl.AllowContentLength(280); !ok {
		t.Error("content length exactly at the limit should be allowed")
	}
	if ok, _ := sl.AllowContentLength(281); ok {
		t.Error("content length over the limit should be rejected")
	}
}
