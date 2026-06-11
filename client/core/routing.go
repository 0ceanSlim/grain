package core

import (
	nostr "github.com/0ceanslim/grain/server/types"
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
// user has no published list.
func (c *Client) RouteFetch(pubkey string) []string {
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
