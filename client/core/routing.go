package core

import (
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

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

// RouteFetch returns the relays to read a user's authored events from: their
// outbox (NIP-65 write) relays, falling back to the index/seed relays when the
// user has no published list. If the fixed-relay override is enabled, the
// pinned read set is used instead (outbox routing off).
func (c *Client) RouteFetch(pubkey string) []string {
	if fixed, read := c.fixedReads(); fixed {
		return read
	}
	if ur := c.ResolveRelays(pubkey); len(ur.Outbox) > 0 {
		return ur.Outbox
	}
	return c.config.IndexRelays
}

// RoutePublish returns the relays an event should be published to under the
// outbox model: the author's own outbox PLUS every p-tagged recipient's inbox
// (their DM inbox for NIP-17 gift wraps), so the event reaches both the
// author's audience and its intended recipients. Falls back to the index/seed
// relays when nothing resolves.
func (c *Client) RoutePublish(event *nostr.Event) []string {
	if fixed, write := c.fixedWrites(); fixed {
		return write
	}

	relays := append([]string(nil), c.ResolveRelays(event.PubKey).Outbox...)

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

	if len(relays) == 0 {
		relays = c.config.IndexRelays
	}
	return relays
}

// SetFixedRelays enables the fixed-relay override: every read uses readRelays
// and every write uses writeRelays, bypassing outbox routing entirely.
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
	log.ClientCore().Warn("Fixed-relay override enabled — outbox routing disabled for this client",
		"read_relays", len(readRelays), "write_relays", len(writeRelays))
}

// ClearFixedRelays disables the override and restores default outbox routing.
func (c *Client) ClearFixedRelays() {
	c.mu.Lock()
	c.fixedMode = false
	c.fixedRead = nil
	c.fixedWrite = nil
	c.mu.Unlock()
	log.ClientCore().Info("Fixed-relay override cleared — outbox routing restored")
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
	return true, append([]string(nil), c.config.IndexRelays...)
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
	return true, append([]string(nil), c.config.IndexRelays...)
}
