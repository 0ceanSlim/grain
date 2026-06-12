package core

import "testing"

func TestMessageRouterOKWaiter(t *testing.T) {
	mr := NewMessageRouter()
	ch := mr.RegisterOKWaiter("evt1", 2)

	mr.RouteOK("evt1", OKResult{Relay: "wss://a", Accepted: true})
	select {
	case ok := <-ch:
		if ok.Relay != "wss://a" || !ok.Accepted {
			t.Errorf("routed OK = %+v", ok)
		}
	default:
		t.Fatal("expected OK to be routed to the waiter")
	}

	// Unknown event id and post-unregister routing must be safe no-ops.
	mr.RouteOK("nonexistent", OKResult{Relay: "x"})
	mr.UnregisterOKWaiter("evt1")
	mr.RouteOK("evt1", OKResult{Relay: "y"})
}

func TestCollectOKResponses(t *testing.T) {
	results := []BroadcastResult{
		{RelayURL: "wss://a", Success: true},
		{RelayURL: "wss://b", Success: true},
		{RelayURL: "wss://c", Success: false}, // send failed — no OK expected
	}
	okCh := make(chan OKResult, 3)
	okCh <- OKResult{Relay: "wss://a", Accepted: true}
	okCh <- OKResult{Relay: "wss://b", Accepted: false, Reason: "blocked: spam"}

	collectOKResponses(okCh, results)

	if !results[0].Accepted {
		t.Error("relay a should be accepted")
	}
	if results[1].Accepted || results[1].Reason != "blocked: spam" {
		t.Errorf("relay b should be rejected with reason, got %+v", results[1])
	}
	if results[2].Accepted {
		t.Error("relay c (send failed) should not be marked accepted")
	}
}
