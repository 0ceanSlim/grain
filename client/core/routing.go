package core

import (
	nostr "github.com/0ceanslim/grain/server/types"
)

// isMetadataKind reports whether an event kind is the sort that lives on the
// indexer relays for everyone to fetch: profile metadata (NIP-01 kind 0) and
// the relay-list events (NIP-65 10002, NIP-17 10050).
func isMetadataKind(kind int) bool {
	return kind == 0 || kind == 10002 || kind == 10050
}

// pTaggedPubkeys returns the distinct pubkeys referenced by the event's `p`
// tags, in order — the recipients a reply / mention / DM is directed at.
func pTaggedPubkeys(event *nostr.Event) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "p" && tag[1] != "" {
			if _, ok := seen[tag[1]]; ok {
				continue
			}
			seen[tag[1]] = struct{}{}
			out = append(out, tag[1])
		}
	}
	return out
}

// routePrefs are the routing preferences one route computation uses: the
// Client's own (app-wide defaults) or a UserContext's, which override them.
type routePrefs struct {
	fixed      bool
	fixedRead  []string
	fixedWrite []string
	index      []string // effective RoleIndexer set
	broadcast  []string // RoleBroadcast mirrors for writes
}

// prefs returns the Client's app-wide routing preferences.
func (c *Client) prefs() routePrefs {
	p := routePrefs{index: c.indexRelays(), broadcast: c.AppRelays(RoleBroadcast)}
	p.fixed, p.fixedRead = c.fixedReads()
	_, p.fixedWrite = c.fixedWrites()
	return p
}

// RouteFetch returns the relays to read a user's authored events from: their
// outbox (NIP-65 write) relays, falling back to the index/seed relays when the
// user has no published list. If the fixed-relay override is enabled, the
// pinned read set is used instead (outbox routing off).
func (c *Client) RouteFetch(pubkey string) []string { return c.routeFetch(c.prefs(), pubkey) }

func (c *Client) routeFetch(p routePrefs, pubkey string) []string {
	if p.fixed {
		return p.fixedRead
	}
	if ur := c.ResolveRelays(pubkey); len(ur.Outbox) > 0 {
		return ur.Outbox
	}
	return p.index
}

// RouteMetadata returns the relays to fetch a user's profile/metadata (kind 0)
// and replaceable lists from.
//
// Flow: the index/profile-indexer relays aggregate everyone's metadata and are
// already connected, so they are always queried (fast, reliable). The user's
// own outbox often carries a fresher copy, so it is added too — but only when
// it is ALREADY cached, never via a blocking resolve. Resolving every profile
// view synchronously is what made the dashboard crawl; the cache is warmed by
// the relay-list lookups that happen anyway, so subsequent views include the
// outbox for free. Honours the fixed-relay override.
func (c *Client) RouteMetadata(pubkey string) []string { return c.routeMetadata(c.prefs(), pubkey) }

func (c *Client) routeMetadata(p routePrefs, pubkey string) []string {
	if p.fixed {
		return p.fixedRead
	}
	relays := append([]string(nil), p.index...)
	if ur, ok := c.directory.Cached(pubkey); ok {
		relays = appendUnique(relays, ur.Outbox)
	}
	return relays
}

// RoutePublish returns the relays an event should be published to under the
// outbox model: the author's own outbox PLUS every p-tagged recipient's inbox
// (their DM inbox for NIP-17 gift wraps), so the event reaches both the
// author's audience and its intended recipients. Falls back to the index/seed
// relays when nothing resolves.
func (c *Client) RoutePublish(event *nostr.Event) []string { return c.routePublish(c.prefs(), event) }

func (c *Client) routePublish(p routePrefs, event *nostr.Event) []string {
	if p.fixed {
		return p.fixedWrite
	}

	relays := append([]string(nil), c.ResolveRelays(event.PubKey).Outbox...)

	// Profile metadata (kind 0) and relay lists (10002 / 10050) belong on the
	// indexer relays too — that's where clients fetch them for arbitrary users,
	// so a profile update must reach the indexers, not just the author's outbox.
	if isMetadataKind(event.Kind) {
		relays = appendUnique(relays, p.index)
	}

	for _, pk := range pTaggedPubkeys(event) {
		if pk == event.PubKey {
			continue
		}
		ur := c.ResolveRelays(pk)
		inbox := ur.Inbox
		if event.Kind == 1059 && len(ur.DMInbox) > 0 { // NIP-17 gift wrap → DM inbox
			inbox = ur.DMInbox
		}
		relays = appendUnique(relays, inbox)
	}

	// Mirror writes to the broadcast relays ("event blasters"), if set, so a
	// post fans out beyond the author's own outbox.
	relays = appendUnique(relays, p.broadcast)

	if len(relays) == 0 {
		relays = p.index
	}
	return relays
}

// SetFixedRelays enables the fixed-relay override: every read uses readRelays
// and every write uses writeRelays, bypassing outbox routing entirely. On the
// Client this is the app-wide default; a [UserContext] can override it for
// one user with [UserContext.PinFixedRelays].
//
// This DISABLES the outbox model — replies will not reach other users' inbox
// relays — and is intended only for users who explicitly want a fixed- or
// single-relay client. It is off by default and not recommended.
func (c *Client) SetFixedRelays(readRelays, writeRelays []string) {
	c.mu.Lock()
	c.fixedMode = true
	c.fixedRead = append([]string(nil), readRelays...)
	c.fixedWrite = append([]string(nil), writeRelays...)
	c.mu.Unlock()
	clog().Warn("Fixed-relay override enabled — outbox routing disabled for this client",
		"read_relays", len(readRelays), "write_relays", len(writeRelays))
}

// ClearFixedRelays disables the override and restores default outbox routing.
func (c *Client) ClearFixedRelays() {
	c.mu.Lock()
	c.fixedMode = false
	c.fixedRead = nil
	c.fixedWrite = nil
	c.mu.Unlock()
	clog().Info("Fixed-relay override cleared — outbox routing restored")
}

// FixedRelaysEnabled reports whether the fixed-relay override is active.
func (c *Client) FixedRelaysEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.fixedMode
}

// fixedReads returns the pinned read set when the override is on (a copy, so the
// lock isn't held during routing). The boolean reports whether the override is
// active. An empty pinned set falls back to index relays — never to outbox,
// since the whole point of the override is to stay off the outbox model.
func (c *Client) fixedReads() (bool, []string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.fixedMode {
		return false, nil
	}
	if len(c.fixedRead) > 0 {
		return true, append([]string(nil), c.fixedRead...)
	}
	return true, append([]string(nil), c.indexRelays()...)
}

// fixedWrites is the write-side counterpart of fixedReads.
func (c *Client) fixedWrites() (bool, []string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.fixedMode {
		return false, nil
	}
	if len(c.fixedWrite) > 0 {
		return true, append([]string(nil), c.fixedWrite...)
	}
	return true, append([]string(nil), c.indexRelays()...)
}
