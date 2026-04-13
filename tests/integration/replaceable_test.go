package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// These tests verify NIP-01 replaceable / addressable semantics: after the
// vendored nostrdb fork exposes a real delete, grain's storeReplaceable /
// storeAddressable paths remove the superseded version before ingesting the
// new one, so even limit-aware / until-cursor queries see exactly one copy.

func TestReplaceable_OnlyLatestVisible(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	// SignEvent uses time.Now().Unix() at second granularity, so we have to
	// build these manually with ascending created_at to avoid the same-ts
	// tiebreak path.
	base := time.Now().Unix() - 10
	for i := 0; i < 3; i++ {
		evt := kp.SignEventAt(0, fmt.Sprintf(`{"name":"v%d"}`, i), nil, base+int64(i))
		c.SendEvent(evt)
		if ok, reason := c.ExpectOK(evt.ID, 3*time.Second); !ok {
			t.Fatalf("kind-0 v%d rejected: %q", i, reason)
		}
	}

	sub := tests.RandomSubID()
	c.Subscribe(sub, map[string]interface{}{
		"authors": []string{kp.PubKey},
		"kinds":   []int{0},
	})
	got := c.ExpectEOSE(sub, 3*time.Second)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 replaceable result, got %d", len(got))
	}
	if content, _ := got[0]["content"].(string); content != `{"name":"v2"}` {
		t.Fatalf("expected latest version content, got %q", content)
	}
}

func TestReplaceable_FollowListCollapses(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	base := time.Now().Unix() - 5
	for i := 0; i < 2; i++ {
		evt := kp.SignEventAt(3, "", [][]string{{"p", fmt.Sprintf("%064d", i)}}, base+int64(i))
		c.SendEvent(evt)
		if ok, reason := c.ExpectOK(evt.ID, 3*time.Second); !ok {
			t.Fatalf("kind-3 v%d rejected: %q", i, reason)
		}
	}

	sub := tests.RandomSubID()
	c.Subscribe(sub, map[string]interface{}{
		"authors": []string{kp.PubKey},
		"kinds":   []int{3},
	})
	got := c.ExpectEOSE(sub, 3*time.Second)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 kind-3 result, got %d", len(got))
	}
}

func TestAddressable_GroupsByDTag(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClient(t)
	defer c.Close()

	// Two distinct d-tags, each published twice. Expect 2 results total —
	// the newest per (d) coordinate.
	base := time.Now().Unix() - 5
	dA := "addr-a-" + tests.RandomSubID()
	dB := "addr-b-" + tests.RandomSubID()

	publish := func(dTag, payload string, ts int64) {
		evt := kp.SignEventAt(30000, payload, [][]string{{"d", dTag}}, ts)
		c.SendEvent(evt)
		if ok, reason := c.ExpectOK(evt.ID, 3*time.Second); !ok {
			t.Fatalf("addressable publish (%s,%s) rejected: %q", dTag, payload, reason)
		}
	}

	publish(dA, "a-old", base)
	publish(dA, "a-new", base+1)
	publish(dB, "b-old", base+2)
	publish(dB, "b-new", base+3)

	sub := tests.RandomSubID()
	c.Subscribe(sub, map[string]interface{}{
		"authors": []string{kp.PubKey},
		"kinds":   []int{30000},
		"#d":      []string{dA, dB},
	})
	got := c.ExpectEOSE(sub, 3*time.Second)
	if len(got) != 2 {
		t.Fatalf("expected 2 addressable results (one per d-tag), got %d", len(got))
	}
	// Both surviving events should be the -new versions.
	for _, e := range got {
		content, _ := e["content"].(string)
		if content != "a-new" && content != "b-new" {
			t.Fatalf("expected latest per d-tag, got content %q", content)
		}
	}
}
