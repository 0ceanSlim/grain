package validation

import (
	"testing"
	"time"

	cfgType "github.com/0ceanslim/grain/config/types"
)

// ResolveTimeBounds is the shared source for both timestamp enforcement and the
// NIP-11 created_at limits, so the advertised bounds match what's enforced.
func TestResolveTimeBounds(t *testing.T) {
	// Absolute values are returned verbatim.
	cfg := &cfgType.ServerConfig{}
	cfg.EventTimeConstraints.MinCreatedAt = 1_600_000_000
	cfg.EventTimeConstraints.MaxCreatedAt = 1_700_000_000
	if min, max := ResolveTimeBounds(cfg); min != 1_600_000_000 || max != 1_700_000_000 {
		t.Fatalf("absolute bounds not returned verbatim: min=%d max=%d", min, max)
	}

	// Unset -> defaults: min is the fixed 2020 floor, max is ~now+5m.
	min, max := ResolveTimeBounds(&cfgType.ServerConfig{})
	if min != defaultMinCreatedAt {
		t.Errorf("expected default min floor %d, got %d", defaultMinCreatedAt, min)
	}
	now := time.Now().Unix()
	if max < now || max > now+int64(defaultMaxOffset.Seconds())+5 {
		t.Errorf("expected default max ~now+5m, got %d (now=%d)", max, now)
	}

	// Relative "now-5m" resolves to roughly five minutes ago (recomputed live).
	rel := &cfgType.ServerConfig{}
	rel.EventTimeConstraints.MinCreatedAtString = "now-5m"
	relMin, _ := ResolveTimeBounds(rel)
	if diff := relMin - time.Now().Add(-5*time.Minute).Unix(); diff < -5 || diff > 5 {
		t.Errorf("expected min ~now-5m, off by %d seconds", diff)
	}
}
