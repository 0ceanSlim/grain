package connection

import (
	"sync"
	"time"

	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/stream"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ownEventKinds are the logged-in user's replaceable kinds grain live-syncs, so
// a change made in another client shows up here without a reload.
var ownEventKinds = []int{0, 10002, 10050, 10063, 10096, 10006, 10007, 10012}

var (
	liveSyncMu   sync.Mutex
	liveSyncSubs = map[string]*core.Subscription{}
)

// StartLiveSync opens a subscription to a user's own replaceable events created
// from now on; for each, it drops the cached resolution that event supersedes
// and pushes a live update to the user's open pages. Wired to the SSE hub's
// first-connection hook, so it runs only while a page is open.
func StartLiveSync(pubkey string) {
	cc := GetCoreClient()
	if cc == nil {
		return
	}

	liveSyncMu.Lock()
	if _, running := liveSyncSubs[pubkey]; running {
		liveSyncMu.Unlock()
		return
	}
	liveSyncMu.Unlock()

	since := time.Now()
	relays := cc.OwnListRelays(pubkey)
	sub, err := cc.Subscribe([]nostr.Filter{{
		Authors: []string{pubkey},
		Kinds:   ownEventKinds,
		Since:   &since,
	}}, relays)
	if err != nil {
		log.ClientConnection().Warn("Live-sync subscribe failed", "pubkey", pubkey, "error", err)
		return
	}

	liveSyncMu.Lock()
	// Lost a race with another first-connection — keep the existing sub.
	if _, running := liveSyncSubs[pubkey]; running {
		liveSyncMu.Unlock()
		sub.Close()
		return
	}
	liveSyncSubs[pubkey] = sub
	liveSyncMu.Unlock()

	log.ClientConnection().Info("Live-sync started", "pubkey", pubkey, "relay_count", len(relays))

	go func() {
		// range ends when StopLiveSync closes the subscription (Close closes Events).
		for ev := range sub.Events {
			if ev != nil {
				handleOwnEvent(cc, pubkey, ev)
			}
		}
	}()
}

// StopLiveSync closes a user's live-sync subscription — wired to the SSE hub's
// last-connection hook.
func StopLiveSync(pubkey string) {
	liveSyncMu.Lock()
	sub := liveSyncSubs[pubkey]
	delete(liveSyncSubs, pubkey)
	liveSyncMu.Unlock()
	if sub != nil {
		sub.Close()
		log.ClientConnection().Info("Live-sync stopped", "pubkey", pubkey)
	}
}

// handleOwnEvent drops the cache the event supersedes and notifies the user's
// open pages which kind changed so they can re-render.
func handleOwnEvent(cc *core.Client, pubkey string, ev *nostr.Event) {
	switch ev.Kind {
	case 10063, 10096:
		cc.InvalidateMediaServers(pubkey)
	case 10002, 10050:
		cc.InvalidateUserRelays(pubkey)
	}
	switch ev.Kind {
	case 10002, 10050, 10006, 10007, 10012:
		cc.InvalidateUserRelayLists(pubkey)
	}
	log.ClientConnection().Debug("Live-sync own-event update", "pubkey", pubkey, "kind", ev.Kind)
	stream.Default().Push(pubkey, stream.Message{Type: "list-updated", Kind: ev.Kind})
}
