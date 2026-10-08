package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// UserContext is the per-user handle a downstream app uses to act as one user
// on the outbox engine: their editable relay config ([SessionRelays]), an
// optional [Signer] for publishing, and the shared [Client] that owns the
// connection pool. Construct it with [Client.NewUserContext].
//
// Everything that belongs to one user lives here, not on the Client, so one
// Client can serve many users at once: the relays held open for them
// ([UserContext.HoldRelays]), their routing preferences
// ([UserContext.PinFixedRelays], [UserContext.SetAppRelays]) and their NIP-42
// authenticated connections ([UserContext.Authenticate]), which only ever
// carry this user's requests. Requests made through a UserContext
// (Subscribe, QueryEvents, PublishEvent, …) use those connections where the
// user has authenticated and the Client's shared pool everywhere else. Call
// [UserContext.Close] when the user leaves.
//
// grain's own web layer (client/api, client/session, …) is a reference consumer
// of this surface; a CLI or bot uses it directly. Read-only callers omit the
// signer.
//
// See docs/client-library-guide.md.
type UserContext struct {
	client *Client
	pubkey string
	signer Signer
	relays *SessionRelays

	mu sync.Mutex
	// laneP is this user's private pool, holding only connections the user
	// NIP-42 authenticates on. Created on first use; laneHeld are the urls
	// this context holds a lease on there.
	laneP    *RelayPool
	laneHeld map[string]bool
	// held are shared-pool relays leased for this user (HoldRelays).
	held []string
	// Per-user routing overrides. fixedSet means the user chose fixed-relay
	// mode on or off; otherwise the Client's setting applies. appRelays holds
	// per-role overrides of the Client's app relays.
	fixedSet   bool
	fixedOn    bool
	fixedRead  []string
	fixedWrite []string
	appRelays  map[Role][]string
	closed     bool
}

// UserOption configures a [UserContext] at construction.
type UserOption func(*UserContext)

// WithSigner attaches a signer so the context can publish. Without one the
// context is read-only and [UserContext.Sign] / [UserContext.SignAndPublish]
// return an error.
func WithSigner(s Signer) UserOption {
	return func(uc *UserContext) { uc.signer = s }
}

// NewUserContext creates a [UserContext] for pubkey (64-char hex) on this
// client. Apply options such as [WithSigner] to attach a signer.
func (c *Client) NewUserContext(pubkey string, opts ...UserOption) *UserContext {
	uc := &UserContext{
		client:    c,
		pubkey:    pubkey,
		relays:    newSessionRelays(),
		laneHeld:  make(map[string]bool),
		appRelays: make(map[Role][]string),
	}
	for _, o := range opts {
		o(uc)
	}
	return uc
}

// PublicKey returns the context user's pubkey (hex).
func (uc *UserContext) PublicKey() string { return uc.pubkey }

// Client returns the shared client that owns the connection pool.
func (uc *UserContext) Client() *Client { return uc.client }

// Signer returns the attached signer, or nil for a read-only context.
func (uc *UserContext) Signer() Signer { return uc.signer }

// Relays returns the user's editable, role-tagged session relay config.
func (uc *UserContext) Relays() *SessionRelays { return uc.relays }

// Sign signs event as this user. It requires a signer and errors if the signer's
// public key does not match the context user, so a caller can't accidentally
// sign as someone else.
func (uc *UserContext) Sign(event *nostr.Event) error {
	if uc.signer == nil {
		return fmt.Errorf("user context for %s is read-only: no signer attached", uc.pubkey)
	}
	if pk := uc.signer.PublicKey(); pk != uc.pubkey {
		return fmt.Errorf("signer pubkey %s does not match user context %s", pk, uc.pubkey)
	}
	return uc.signer.SignEvent(event)
}

// Publish routes an already-signed event under the outbox model (the author's
// outbox ∪ each p-tagged recipient's inbox; metadata also to the indexers),
// honouring this user's routing preferences, and broadcasts it as this user,
// returning the per-relay results.
func (uc *UserContext) Publish(ctx context.Context, event *nostr.Event) ([]BroadcastResult, error) {
	return uc.PublishEvent(ctx, event, uc.RoutePublish(event))
}

// SignAndPublish signs event as this user and then publishes it.
func (uc *UserContext) SignAndPublish(ctx context.Context, event *nostr.Event) ([]BroadcastResult, error) {
	if err := uc.Sign(event); err != nil {
		return nil, err
	}
	return uc.Publish(ctx, event)
}

// PinFixedRelays enables the fixed-relay override for this user — their reads
// come from readRelays, their writes go to writeRelays — which DISABLES the
// outbox model for them. Discouraged; for users who explicitly want a
// fixed-/single-relay client. Other users of the Client are unaffected.
func (uc *UserContext) PinFixedRelays(readRelays, writeRelays []string) {
	uc.mu.Lock()
	uc.fixedSet, uc.fixedOn = true, true
	uc.fixedRead = normalizeRelayURLs(readRelays)
	uc.fixedWrite = normalizeRelayURLs(writeRelays)
	uc.mu.Unlock()
}

// ClearFixedRelays turns the fixed-relay override off for this user, restoring
// outbox routing for them even if the Client's app-wide default is fixed.
func (uc *UserContext) ClearFixedRelays() {
	uc.mu.Lock()
	uc.fixedSet, uc.fixedOn = true, false
	uc.fixedRead, uc.fixedWrite = nil, nil
	uc.mu.Unlock()
}

// FixedRelaysEnabled reports whether the fixed-relay override applies to this
// user: their own choice if they made one, else the Client's default.
func (uc *UserContext) FixedRelaysEnabled() bool { return uc.prefs().fixed }

// AppRelays returns this user's relays for a locally-configured role: their
// override if set, else the Client's app-wide setting.
func (uc *UserContext) AppRelays(role Role) []string {
	uc.mu.Lock()
	v, ok := uc.appRelays[role]
	uc.mu.Unlock()
	if ok {
		return append([]string(nil), v...)
	}
	return uc.client.AppRelays(role)
}

// SetAppRelays overrides a locally-configured role's relays for this user only;
// an empty urls removes the override so the Client's setting applies again.
func (uc *UserContext) SetAppRelays(role Role, urls []string) {
	urls = normalizeRelayURLs(urls)
	uc.mu.Lock()
	if len(urls) == 0 {
		delete(uc.appRelays, role)
	} else {
		uc.appRelays[role] = urls
	}
	uc.mu.Unlock()
}

// prefs layers this user's routing overrides over the Client's defaults.
func (uc *UserContext) prefs() routePrefs {
	p := uc.client.prefs()
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if v, ok := uc.appRelays[RoleIndexer]; ok {
		p.index = append([]string(nil), v...)
	}
	if v, ok := uc.appRelays[RoleBroadcast]; ok {
		p.broadcast = append([]string(nil), v...)
	}
	if uc.fixedSet {
		p.fixed = uc.fixedOn
		p.fixedRead, p.fixedWrite = nil, nil
		if uc.fixedOn {
			// As on the Client, an empty pinned set falls back to the index
			// relays, never to outbox routing.
			p.fixedRead = append([]string(nil), uc.fixedRead...)
			if len(p.fixedRead) == 0 {
				p.fixedRead = append([]string(nil), p.index...)
			}
			p.fixedWrite = append([]string(nil), uc.fixedWrite...)
			if len(p.fixedWrite) == 0 {
				p.fixedWrite = append([]string(nil), p.index...)
			}
		}
	}
	return p
}

// RouteFetch is [Client.RouteFetch] under this user's routing preferences.
func (uc *UserContext) RouteFetch(pubkey string) []string {
	return uc.client.routeFetch(uc.prefs(), pubkey)
}

// RouteMetadata is [Client.RouteMetadata] under this user's routing preferences.
func (uc *UserContext) RouteMetadata(pubkey string) []string {
	return uc.client.routeMetadata(uc.prefs(), pubkey)
}

// RoutePublish is [Client.RoutePublish] under this user's routing preferences.
func (uc *UserContext) RoutePublish(event *nostr.Event) []string {
	return uc.client.routePublish(uc.prefs(), event)
}

// Subscribe is [Client.Subscribe] as this user: relays the user has NIP-42
// authenticated to are reached over their own connection.
func (uc *UserContext) Subscribe(ctx context.Context, filters []nostr.Filter, relays []string) (*Subscription, error) {
	return uc.client.subscribe(ctx, uc, filters, relays)
}

// StreamEvents is [Client.StreamEvents] as this user.
func (uc *UserContext) StreamEvents(ctx context.Context, filter nostr.Filter, relays []string, opts ...StreamOption) <-chan *nostr.Event {
	return uc.client.streamEvents(ctx, uc, filter, relays, opts...)
}

// QueryEvents is [Client.QueryEvents] as this user.
func (uc *UserContext) QueryEvents(ctx context.Context, filter nostr.Filter, relays []string, opts ...StreamOption) []*nostr.Event {
	return uc.client.queryEvents(ctx, uc, filter, relays, opts...)
}

// FetchEvents is [Client.FetchEvents] as this user.
func (uc *UserContext) FetchEvents(ctx context.Context, filters []nostr.Filter, relays []string, limit int, timeout time.Duration) []*nostr.Event {
	return uc.client.fetchEvents(ctx, uc, filters, relays, limit, timeout)
}

// PublishEvent broadcasts an already-signed event to relays as this user (to
// [UserContext.RoutePublish] when relays is empty).
func (uc *UserContext) PublishEvent(ctx context.Context, event *nostr.Event, relays []string) ([]BroadcastResult, error) {
	if event == nil {
		return nil, &ClientError{Message: "event cannot be nil"}
	}
	if len(relays) == 0 {
		relays = uc.RoutePublish(event)
	}
	if len(relays) == 0 {
		return nil, &ClientError{Message: "no relays available for publishing"}
	}
	return uc.client.broadcast(ctx, uc, event, relays, 0), nil
}

// PublishEventWithRetry is [UserContext.PublishEvent] with per-relay retries.
func (uc *UserContext) PublishEventWithRetry(ctx context.Context, event *nostr.Event, relays []string, maxRetries int) ([]BroadcastResult, error) {
	if event == nil {
		return nil, &ClientError{Message: "event cannot be nil"}
	}
	if len(relays) == 0 {
		relays = uc.RoutePublish(event)
	}
	if len(relays) == 0 {
		return nil, &ClientError{Message: "no relays available for publishing"}
	}
	if maxRetries < 1 {
		maxRetries = 1
	}
	return uc.client.broadcast(ctx, uc, event, relays, maxRetries), nil
}

// PublishEventStream is [Client.PublishEventStream] as this user.
func (uc *UserContext) PublishEventStream(ctx context.Context, event *nostr.Event, relays []string) <-chan BroadcastResult {
	return uc.client.broadcastStream(ctx, uc, event, relays)
}

// HoldRelays keeps relays connected in the shared pool for as long as this
// user holds them, replacing whatever this user held before (other users'
// holds are untouched). Released holds become idle-evictable. Returns an
// error only if none of a non-empty set could be connected.
func (uc *UserContext) HoldRelays(urls []string) error {
	urls = normalizeRelayURLs(urls)
	pool := uc.client.relayPool

	uc.mu.Lock()
	if uc.closed {
		uc.mu.Unlock()
		return fmt.Errorf("user context for %s is closed", uc.pubkey)
	}
	previous := uc.held
	uc.held = nil
	uc.mu.Unlock()

	acquired := make([]string, 0, len(urls))
	for _, u := range urls {
		if _, err := pool.Acquire(u); err != nil {
			clog().Debug("Failed to acquire user relay", "relay", u, "pubkey", uc.pubkey, "error", err)
			continue
		}
		acquired = append(acquired, u)
	}
	// Release after acquiring, so relays in both sets never drop to zero
	// leases in between.
	for _, u := range previous {
		pool.Release(u)
	}

	uc.mu.Lock()
	if uc.closed {
		// Closed while we were dialing: give the new holds straight back.
		uc.mu.Unlock()
		for _, u := range acquired {
			pool.Release(u)
		}
		return fmt.Errorf("user context for %s is closed", uc.pubkey)
	}
	uc.held = acquired
	uc.mu.Unlock()

	if len(acquired) == 0 && len(urls) > 0 {
		return fmt.Errorf("failed to connect to any of the %d requested relays", len(urls))
	}
	return nil
}

// AddHeldRelay adds url to the relays this user holds, connecting it if
// needed. Other users' holds are untouched.
func (uc *UserContext) AddHeldRelay(url string) error {
	norm, ok := normalizeRelayURL(url)
	if !ok {
		return fmt.Errorf("invalid relay url: %q", url)
	}
	uc.mu.Lock()
	if uc.closed {
		uc.mu.Unlock()
		return fmt.Errorf("user context for %s is closed", uc.pubkey)
	}
	for _, u := range uc.held {
		if u == norm {
			uc.mu.Unlock()
			return nil
		}
	}
	uc.mu.Unlock()

	if _, err := uc.client.relayPool.Acquire(norm); err != nil {
		return err
	}
	uc.mu.Lock()
	if uc.closed {
		uc.mu.Unlock()
		uc.client.relayPool.Release(norm)
		return fmt.Errorf("user context for %s is closed", uc.pubkey)
	}
	uc.held = append(uc.held, norm)
	uc.mu.Unlock()
	return nil
}

// DropHeldRelay releases this user's hold on url. The connection stays up for
// anyone else using it and becomes idle-evictable once nobody is.
func (uc *UserContext) DropHeldRelay(url string) {
	norm, ok := normalizeRelayURL(url)
	if !ok {
		return
	}
	uc.mu.Lock()
	found := false
	for i, u := range uc.held {
		if u == norm {
			uc.held = append(uc.held[:i], uc.held[i+1:]...)
			found = true
			break
		}
	}
	uc.mu.Unlock()
	if found {
		uc.client.relayPool.Release(norm)
	}
}

// HeldRelays returns the shared-pool relays this user currently holds.
func (uc *UserContext) HeldRelays() []string {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	return append([]string(nil), uc.held...)
}

// ReleaseRelays drops all of this user's holds (see [UserContext.HoldRelays]).
func (uc *UserContext) ReleaseRelays() {
	uc.mu.Lock()
	previous := uc.held
	uc.held = nil
	uc.mu.Unlock()
	for _, u := range previous {
		uc.client.relayPool.Release(u)
	}
}

// Close ends this user's presence on the Client: it releases their relay
// holds and closes their authenticated connections, ending those NIP-42
// sessions. The context stays usable for anonymous-routed requests, but can
// no longer hold relays or authenticate.
func (uc *UserContext) Close() error {
	uc.mu.Lock()
	if uc.closed {
		uc.mu.Unlock()
		return nil
	}
	uc.closed = true
	lane := uc.laneP
	uc.laneP = nil
	uc.laneHeld = make(map[string]bool)
	uc.mu.Unlock()

	uc.ReleaseRelays()
	if lane != nil {
		uc.client.lanesMu.Lock()
		delete(uc.client.lanes, uc)
		uc.client.lanesMu.Unlock()
		return lane.Close()
	}
	return nil
}

// lane returns this user's private pool, creating it on first use. After
// Close it returns a pool with no connections that is never stored, so
// nothing can be left open.
func (uc *UserContext) lane() *RelayPool {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	if uc.laneP == nil {
		p := NewRelayPool(uc.client.config)
		if uc.closed {
			return p
		}
		uc.laneP = p
		uc.client.lanesMu.Lock()
		uc.client.lanes[uc] = struct{}{}
		uc.client.lanesMu.Unlock()
	}
	return uc.laneP
}

// laneIfAny returns this user's private pool, or nil if it hasn't been made.
func (uc *UserContext) laneIfAny() *RelayPool {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	return uc.laneP
}

// holdLaneConn returns this user's own live connection to url, dialing it in
// their private pool (and holding a lease on it) if needed.
func (uc *UserContext) holdLaneConn(url string) (*RelayConnection, error) {
	uc.mu.Lock()
	closed := uc.closed
	uc.mu.Unlock()
	if closed {
		return nil, fmt.Errorf("user context for %s is closed", uc.pubkey)
	}
	lane := uc.lane()

	uc.mu.Lock()
	held := uc.laneHeld[url]
	uc.mu.Unlock()
	if held {
		if conn, err := lane.GetConnection(url); err == nil && conn.getStatus() == StatusConnected {
			return conn, nil
		}
		// The held connection died; its lease went with it. Redial below.
	}

	conn, err := lane.Acquire(url)
	if err != nil {
		return nil, err
	}
	uc.mu.Lock()
	uc.laneHeld[url] = true
	uc.mu.Unlock()
	return conn, nil
}

// StreamNotes streams author's text notes (kind 1) from their outbox relays as
// each relay answers — the lazy-hydration path for a profile feed. Pass options
// like [WithLimit] to bound it. Routing honours this user's preferences.
func (uc *UserContext) StreamNotes(ctx context.Context, author string, opts ...StreamOption) <-chan *nostr.Event {
	filter := nostr.Filter{Authors: []string{author}, Kinds: []int{1}}
	return uc.StreamEvents(ctx, filter, uc.RouteFetch(author), opts...)
}

// FetchNotes collects author's text notes (kind 1) from their outbox relays.
// Best-effort: per-relay failures are logged, so an empty result means "none
// found" rather than a hard error. For incremental delivery use [StreamNotes].
func (uc *UserContext) FetchNotes(ctx context.Context, author string, opts ...StreamOption) []*nostr.Event {
	filter := nostr.Filter{Authors: []string{author}, Kinds: []int{1}}
	return uc.QueryEvents(ctx, filter, uc.RouteFetch(author), opts...)
}

// Reply builds a NIP-10 kind-1 reply to parent, signs it as this user, and
// publishes it under the outbox model so it reaches the parent author's inbox as
// well as the user's own audience. It returns the signed reply and the per-relay
// broadcast results. Requires a signer.
func (uc *UserContext) Reply(ctx context.Context, parent *nostr.Event, content string) (*nostr.Event, []BroadcastResult, error) {
	if parent == nil {
		return nil, nil, fmt.Errorf("reply: parent event is nil")
	}
	evt := &nostr.Event{
		Kind:      1,
		Content:   content,
		CreatedAt: time.Now().Unix(),
		Tags:      buildReplyTags(parent),
	}
	results, err := uc.SignAndPublish(ctx, evt)
	return evt, results, err
}

// buildReplyTags assembles the NIP-10 e/p tags for a reply to parent: the thread
// root and the immediate parent as marked "e" tags, plus "p" tags for everyone
// already in the thread and the parent's author, so the whole thread is notified.
func buildReplyTags(parent *nostr.Event) [][]string {
	var tags [][]string

	// Thread root: a marked "root" e-tag on the parent, else the first "e" tag
	// (positional NIP-10), else the parent itself is the root.
	root := ""
	for _, tag := range parent.Tags {
		if len(tag) >= 4 && tag[0] == "e" && tag[3] == "root" {
			root = tag[1]
			break
		}
	}
	if root == "" {
		for _, tag := range parent.Tags {
			if len(tag) >= 2 && tag[0] == "e" {
				root = tag[1]
				break
			}
		}
	}
	if root != "" && root != parent.ID {
		tags = append(tags, []string{"e", root, "", "root"})
		tags = append(tags, []string{"e", parent.ID, "", "reply"})
	} else {
		tags = append(tags, []string{"e", parent.ID, "", "root"})
	}

	// Notify everyone already in the thread, plus the parent's author.
	seen := make(map[string]struct{})
	addP := func(pk string) {
		if pk == "" {
			return
		}
		if _, ok := seen[pk]; ok {
			return
		}
		seen[pk] = struct{}{}
		tags = append(tags, []string{"p", pk})
	}
	for _, tag := range parent.Tags {
		if len(tag) >= 2 && tag[0] == "p" {
			addP(tag[1])
		}
	}
	addP(parent.PubKey)

	return tags
}
