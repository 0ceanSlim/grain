package core

import (
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// FetchEvents collects up to limit distinct events (deduped by id) matching
// filters from the given relays, returning when every relay has sent EOSE, the
// limit is reached, or the timeout elapses. Unlike collectLatestReplaceable
// (which keeps only the single newest), this gathers a batch — used for bulk
// queries like relay-list seeding.
func (c *Client) FetchEvents(filters []nostr.Filter, relays []string, limit int, timeout time.Duration) []*nostr.Event {
	if len(relays) == 0 {
		return nil
	}

	sub, err := c.Subscribe(filters, relays)
	if err != nil {
		log.ClientCore().Debug("FetchEvents subscribe failed", "error", err)
		return nil
	}
	defer sub.Close()

	seen := make(map[string]struct{})
	eose := make(map[string]bool)
	var events []*nostr.Event
	deadline := time.After(timeout)

	for {
		select {
		case ev := <-sub.Events:
			if ev == nil {
				continue
			}
			if _, dup := seen[ev.ID]; dup {
				continue
			}
			seen[ev.ID] = struct{}{}
			events = append(events, ev)
			if limit > 0 && len(events) >= limit {
				return events
			}

		case relayURL := <-sub.EOSE:
			eose[relayURL] = true
			if len(eose) >= len(relays) {
				return events
			}

		case <-sub.Errors:
			// One relay failing isn't fatal; keep collecting from the rest.

		case <-deadline:
			return events
		}
	}
}

// SeedKnownRelays bulk-fetches recent relay-list events (NIP-65 kind 10002 and
// NIP-17 kind 10050) from the index relays and folds every advertised relay
// into the directory, so the "known" set starts broad immediately instead of
// growing only as the user browses. Runs once at startup; safe to re-run to
// refresh. All relay URLs are normalised on the way in (see normalizeRelayURL),
// so near-duplicates collapse.
func (c *Client) SeedKnownRelays() {
	relays := c.config.IndexRelays
	if len(relays) == 0 {
		return
	}

	const limit = 500
	lim := limit
	events := c.FetchEvents(
		[]nostr.Filter{{Kinds: []int{10002, 10050}, Limit: &lim}},
		relays, limit, 8*time.Second,
	)

	stored := 0
	for _, ev := range events {
		switch ev.Kind {
		case 10002:
			mb := parseMailboxEvent(ev)
			c.directory.Store(ev.PubKey, &UserRelays{
				Outbox: appendUnique(mb.Write, mb.Both),
				Inbox:  appendUnique(mb.Read, mb.Both),
			})
			stored++
		case 10050:
			c.directory.Store(ev.PubKey, &UserRelays{DMInbox: parseDMRelays(ev)})
			stored++
		}
	}

	log.ClientCore().Info("Seeded known relays from indexers",
		"events", len(events), "stored", stored, "known", c.PoolStats().Known)
}
