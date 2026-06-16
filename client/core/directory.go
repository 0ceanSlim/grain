package core

import (
	"context"
	"sync"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// UserRelays holds a target user's per-target event-derived relay roles,
// resolved from their published NIP-65 (kind 10002) and NIP-17 (kind 10050)
// events. These are the only roles the directory resolves for arbitrary users;
// the richer self-only roles (search / blocked / favorite / private) load into
// the logged-in user's own session config, not here.
type UserRelays struct {
	Outbox    []string  // NIP-65 write relays — publish / fetch their notes
	Inbox     []string  // NIP-65 read relays — deliver replies / zaps here
	DMInbox   []string  // NIP-17 kind 10050 DM relays
	FetchedAt time.Time // when this was resolved
	Negative  bool      // user has no published lists — cached briefly
}

// RelayDirectory caches per-user relay-role resolutions with a TTL, collapsing
// concurrent lookups for the same pubkey into a single network fetch.
type RelayDirectory struct {
	mu       sync.Mutex
	entries  map[string]*UserRelays
	inflight map[string]chan struct{}
	ttl      time.Duration
	negTTL   time.Duration
	resolve  func(pubkey string) *UserRelays
}

func newRelayDirectory(ttl, negTTL time.Duration, resolve func(string) *UserRelays) *RelayDirectory {
	if ttl <= 0 {
		ttl = time.Hour
	}
	if negTTL <= 0 {
		negTTL = time.Minute
	}
	return &RelayDirectory{
		entries:  make(map[string]*UserRelays),
		inflight: make(map[string]chan struct{}),
		ttl:      ttl,
		negTTL:   negTTL,
		resolve:  resolve,
	}
}

// fresh reports whether a cached entry is still within its TTL. Negative
// results use the shorter TTL so a user who later publishes a list is picked up
// soon instead of being treated as relay-less for an hour.
func (d *RelayDirectory) fresh(ur *UserRelays) bool {
	if ur == nil {
		return false
	}
	ttl := d.ttl
	if ur.Negative {
		ttl = d.negTTL
	}
	return time.Since(ur.FetchedAt) < ttl
}

// Lookup returns the user's relay roles, resolving and caching on a miss or
// stale entry. Concurrent lookups for the same pubkey share one resolution.
func (d *RelayDirectory) Lookup(pubkey string) *UserRelays {
	for {
		d.mu.Lock()
		if ur, ok := d.entries[pubkey]; ok && d.fresh(ur) {
			d.mu.Unlock()
			return ur
		}
		if ch, ok := d.inflight[pubkey]; ok {
			// A resolution is in progress — wait for it, then re-check.
			d.mu.Unlock()
			<-ch
			continue
		}
		ch := make(chan struct{})
		d.inflight[pubkey] = ch
		d.mu.Unlock()

		ur := d.resolve(pubkey)
		if ur == nil {
			ur = &UserRelays{Negative: true}
		}
		// Stamp the fetch time here, not in the resolver, so a resolver that
		// forgets can't silently disable caching (entry never reads as fresh).
		ur.FetchedAt = time.Now()

		d.mu.Lock()
		d.entries[pubkey] = ur
		delete(d.inflight, pubkey)
		close(ch)
		d.mu.Unlock()
		return ur
	}
}

// Invalidate drops a cached entry so the next Lookup re-resolves — e.g. after
// observing a newer relay-list event for the user.
func (d *RelayDirectory) Invalidate(pubkey string) {
	d.mu.Lock()
	delete(d.entries, pubkey)
	d.mu.Unlock()
}

// Store inserts or merges a resolved relay set for a pubkey directly, without a
// network resolve — used by bulk seeding. A NIP-65 (10002) and a NIP-17 (10050)
// event for the same author arrive separately, so non-empty fields merge rather
// than clobber.
func (d *RelayDirectory) Store(pubkey string, ur *UserRelays) {
	if pubkey == "" || ur == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	existing, ok := d.entries[pubkey]
	if !ok {
		ur.FetchedAt = time.Now()
		ur.Negative = false
		d.entries[pubkey] = ur
		return
	}
	if len(ur.Outbox) > 0 {
		existing.Outbox = ur.Outbox
	}
	if len(ur.Inbox) > 0 {
		existing.Inbox = ur.Inbox
	}
	if len(ur.DMInbox) > 0 {
		existing.DMInbox = ur.DMInbox
	}
	existing.FetchedAt = time.Now()
	existing.Negative = false
}

// KnownRelays returns the distinct relay URLs across every resolved entry —
// the union of all users' outbox / inbox / DM relays the directory has seen.
// This is the bulk of the client's "known relays" set: relays we're aware of
// from indexer-seeded mailbox lists, whether or not we're connected to them.
func (d *RelayDirectory) KnownRelays() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	set := make(map[string]struct{})
	for _, ur := range d.entries {
		for _, r := range ur.Outbox {
			set[r] = struct{}{}
		}
		for _, r := range ur.Inbox {
			set[r] = struct{}{}
		}
		for _, r := range ur.DMInbox {
			set[r] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for r := range set {
		out = append(out, r)
	}
	return out
}

// Cached returns a user's resolved relays only if a fresh entry is already in
// the cache, WITHOUT triggering a network resolve. For latency-sensitive paths
// (e.g. rendering a profile) that must not block on resolution.
func (d *RelayDirectory) Cached(pubkey string) (*UserRelays, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if ur, ok := d.entries[pubkey]; ok && d.fresh(ur) {
		return ur, true
	}
	return nil, false
}

// ResolveRelays returns a target user's per-target relay roles (outbox / inbox /
// DM inbox), resolved from their published relay-list events and cached with a
// TTL. Safe for concurrent use; the logged-in user is just another pubkey.
func (c *Client) ResolveRelays(pubkey string) *UserRelays {
	return c.directory.Lookup(pubkey)
}

// WarmRelays kicks an asynchronous resolution of a user's relay lists into the
// directory if not already cached, so their mailbox relays join the "known" set
// and the cache is ready for outbox-routed fetches next time — without blocking
// the caller and without dialing (resolution queries the index relays only).
func (c *Client) WarmRelays(pubkey string) {
	if _, ok := c.directory.Cached(pubkey); ok {
		return
	}
	go c.directory.Lookup(pubkey)
}

// fetchUserRelaysFromNetwork is the RelayDirectory's resolver: it queries the
// index/seed relays for a user's latest kind 10002 (NIP-65) and 10050 (NIP-17)
// events concurrently and maps them to relay roles.
func (c *Client) fetchUserRelaysFromNetwork(pubkey string) *UserRelays {
	relays := c.config.IndexRelays
	if len(relays) == 0 {
		relays = c.GetConnectedRelays()
	}

	var (
		nip65 *nostr.Event
		nip17 *nostr.Event
		wg    sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		nip65 = c.fetchLatestEvent(context.Background(), pubkey, 10002, relays)
	}()
	go func() {
		defer wg.Done()
		nip17 = c.fetchLatestEvent(context.Background(), pubkey, 10050, relays)
	}()
	wg.Wait()

	ur := &UserRelays{FetchedAt: time.Now()}
	if nip65 != nil {
		mb := parseMailboxEvent(nip65)
		ur.Outbox = appendUnique(mb.Write, mb.Both)
		ur.Inbox = appendUnique(mb.Read, mb.Both)
	}
	if nip17 != nil {
		ur.DMInbox = parseDMRelays(nip17)
	}
	ur.Negative = len(ur.Outbox) == 0 && len(ur.Inbox) == 0 && len(ur.DMInbox) == 0

	log.ClientCore().Debug("Resolved user relays", "pubkey", pubkey,
		"outbox", len(ur.Outbox), "inbox", len(ur.Inbox), "dm", len(ur.DMInbox), "negative", ur.Negative)
	return ur
}

// fetchLatestEvent returns the newest event of kind for pubkey from relays, or
// nil if none arrives. Shared by the directory resolver.
func (c *Client) fetchLatestEvent(ctx context.Context, pubkey string, kind int, relays []string) *nostr.Event {
	return c.fetchLatestEventWithin(ctx, pubkey, kind, relays, 5*time.Second)
}

// fetchLatestEventWithin is fetchLatestEvent with a caller-chosen deadline. A
// replaceable event, when it exists, comes back well under a second; the full
// deadline is only ever spent waiting on unresponsive relays in the negative
// case. Callers resolving "does the user have this list at all?" (media servers,
// relay lists) pass a shorter deadline so a cold miss doesn't stall the UI for
// the full 5s.
func (c *Client) fetchLatestEventWithin(ctx context.Context, pubkey string, kind int, relays []string, timeout time.Duration) *nostr.Event {
	if len(relays) == 0 {
		return nil
	}
	limit := 1
	filter := nostr.Filter{
		Authors: []string{pubkey},
		Kinds:   []int{kind},
		Limit:   &limit,
	}
	sub, err := c.Subscribe(ctx, []nostr.Filter{filter}, relays)
	if err != nil {
		log.ClientCore().Debug("Subscribe failed during relay resolution", "pubkey", pubkey, "kind", kind, "error", err)
		return nil
	}
	defer sub.Close()
	return collectLatestReplaceable(ctx, sub, len(relays), timeout)
}

// parseDMRelays extracts relay URLs from a NIP-17 kind 10050 event's relay tags.
func parseDMRelays(event *nostr.Event) []string {
	var relays []string
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "relay" {
			if n, ok := normalizeRelayURL(tag[1]); ok {
				relays = append(relays, n)
			}
		}
	}
	return relays
}

// appendUnique concatenates lists, dropping duplicates while preserving order.
func appendUnique(lists ...[]string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, l := range lists {
		for _, s := range l {
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
