package core

import (
	"encoding/json"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// wireEvent signs evt and returns it the way RouteMessage receives it: the
// generic map a relay's EVENT frame decodes to. mutate, if set, runs on the
// map after signing to simulate a relay tampering with the event.
func wireEvent(t *testing.T, signer *EventSigner, evt nostr.Event, mutate func(map[string]interface{})) map[string]interface{} {
	t.Helper()
	if evt.CreatedAt == 0 {
		evt.CreatedAt = time.Now().Unix()
	}
	if evt.Tags == nil {
		evt.Tags = [][]string{}
	}
	if err := signer.SignEvent(&evt); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(m)
	}
	return m
}

// routeOne registers a subscription with filters, routes one EVENT frame into
// it, and reports whether it was delivered.
func routeOne(t *testing.T, filters []nostr.Filter, data map[string]interface{}) bool {
	t.Helper()
	c := NewClient(DefaultConfig())
	sub := NewSubscription("verify-test", filters, nil, c)
	c.relayPool.RegisterSubscription(sub.ID, sub)
	defer c.relayPool.messageRouter.UnregisterSubscription(sub.ID)

	c.relayPool.messageRouter.RouteMessage(sub.ID, "EVENT", data, "wss://hostile.example")
	select {
	case <-sub.Events:
		return true
	default:
		return false
	}
}

// Regression: events from remote relays were delivered without checking the
// id or signature, so any relay could forge events from any pubkey.
func TestRouteMessage_VerifiesInboundEvents(t *testing.T) {
	signer, err := NewEventSignerFromRandom()
	if err != nil {
		t.Fatal(err)
	}
	save := nostr.Event{Kind: 30078, Tags: [][]string{{"d", "char-1"}}, Content: "hp=10"}
	other, err := NewEventSignerFromRandom()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		mutate  func(map[string]interface{})
		deliver bool
	}{
		{"valid", nil, true},
		{"content tampered", func(m map[string]interface{}) { m["content"] = "hp=9999" }, false},
		{"tags tampered", func(m map[string]interface{}) { m["tags"] = []interface{}{[]interface{}{"d", "char-2"}} }, false},
		{"pubkey swapped", func(m map[string]interface{}) { m["pubkey"] = other.publicKey }, false},
		{"sig zeroed", func(m map[string]interface{}) { m["sig"] = string(make([]byte, 128)) }, false},
		{"sig flipped", func(m map[string]interface{}) {
			sig := []byte(m["sig"].(string))
			if sig[0] == '0' {
				sig[0] = '1'
			} else {
				sig[0] = '0'
			}
			m["sig"] = string(sig)
		}, false},
		{"id recomputed but sig stale", func(m map[string]interface{}) {
			m["content"] = "hp=9999"
			evt := nostr.Event{
				PubKey: m["pubkey"].(string), CreatedAt: int64(m["created_at"].(float64)),
				Kind: 30078, Tags: [][]string{{"d", "char-1"}}, Content: "hp=9999",
			}
			id, _ := ComputeEventID(&evt)
			m["id"] = id
		}, false},
		{"sig missing", func(m map[string]interface{}) { delete(m, "sig") }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := routeOne(t, nil, wireEvent(t, signer, save, tc.mutate))
			if got != tc.deliver {
				t.Fatalf("delivered=%v, want %v", got, tc.deliver)
			}
		})
	}
}

// A relay is not trusted to apply the filter either: a validly signed event
// that doesn't match the REQ (e.g. the wrong character's save) is dropped.
func TestRouteMessage_DropsEventsNotMatchingFilters(t *testing.T) {
	signer, err := NewEventSignerFromRandom()
	if err != nil {
		t.Fatal(err)
	}
	evt := nostr.Event{Kind: 30078, Tags: [][]string{{"d", "char-2"}}, Content: "x"}

	want := NewFilterBuilder().Kinds(30078).Authors(signer.publicKey).Tag("d", "char-1").Build()
	if routeOne(t, []nostr.Filter{want}, wireEvent(t, signer, evt, nil)) {
		t.Fatal("delivered an event whose d tag doesn't match the filter")
	}

	// Filters are OR-ed: matching any one is enough.
	alt := NewFilterBuilder().Tag("#d", "char-2").Build()
	if !routeOne(t, []nostr.Filter{want, alt}, wireEvent(t, signer, evt, nil)) {
		t.Fatal("dropped an event matching the second filter")
	}

	// NIP-50 matching is the relay's call; don't second-guess it locally.
	search := nostr.Filter{Kinds: []int{30078}, Search: "words not in content"}
	if !routeOne(t, []nostr.Filter{search}, wireEvent(t, signer, evt, nil)) {
		t.Fatal("dropped a search result over local search semantics")
	}
}

func TestVerifyEvent(t *testing.T) {
	signer, err := NewEventSignerFromRandom()
	if err != nil {
		t.Fatal(err)
	}
	evt := nostr.Event{Kind: 1, CreatedAt: 1700000000, Tags: [][]string{}, Content: "hi"}
	if err := signer.SignEvent(&evt); err != nil {
		t.Fatal(err)
	}
	if err := VerifyEvent(&evt); err != nil {
		t.Fatalf("valid event rejected: %v", err)
	}
	evt.Content = "bye"
	if VerifyEvent(&evt) == nil {
		t.Fatal("tampered event accepted")
	}
	if VerifyEvent(nil) == nil {
		t.Fatal("nil event accepted")
	}
}
