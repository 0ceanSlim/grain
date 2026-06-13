package core

import (
	"fmt"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// relayListTagName returns the tag name relay URLs use in a given relay-list
// event kind, and whether grain can publish that kind. NIP-65 (10002) uses `r`
// tags with read/write markers; the NIP-17/51 lists use plain `relay` tags.
func relayListTagName(kind int) (string, bool) {
	switch kind {
	case 10002: // NIP-65 read/write relays
		return "r", true
	case 10050, // NIP-17 DM relays
		10006, // NIP-51 blocked relays
		10007, // NIP-51 search relays
		10012: // NIP-51 favorite relays
		return "relay", true
	default:
		return "", false
	}
}

// RelayListEntry is one relay in a relay-list event. Read / Write are only
// meaningful for NIP-65 kind 10002 — an entry that is both (or neither) is
// written unmarked, meaning "read and write". The other kinds ignore the flags.
type RelayListEntry struct {
	URL   string `json:"url"`
	Read  bool   `json:"read"`
	Write bool   `json:"write"`
}

// AssembleRelayListEvent builds an UNSIGNED relay-list event of the given kind
// from the user's existing one, rewriting the relay tags while preserving the
// content and every non-relay tag — the same conservative "don't drop data"
// rule as the profile and media-server editors.
//
// Tag shape by kind:
//   - 10002 (NIP-65): ["r", url] (both), ["r", url, "read"], or ["r", url, "write"]
//   - 10050 / 10006 / 10007 / 10012: ["relay", url]
//
// existing may be nil (a first list). The returned event has no ID or Sig: the
// caller signs it.
func AssembleRelayListEvent(existing *nostr.Event, kind int, pubkey string, entries []RelayListEntry) (*nostr.Event, error) {
	tagName, ok := relayListTagName(kind)
	if !ok {
		return nil, fmt.Errorf("unsupported relay-list kind %d", kind)
	}

	var existingTags [][]string
	content := ""
	if existing != nil {
		existingTags = existing.Tags
		content = existing.Content
		if existing.PubKey != "" {
			pubkey = existing.PubKey
		}
	}

	// Preserve every tag that isn't a relay entry of this kind's tag name.
	tags := make([][]string, 0, len(existingTags)+len(entries))
	for _, t := range existingTags {
		if len(t) >= 1 && t[0] == tagName {
			continue
		}
		tags = append(tags, t)
	}

	// Append the new relay list in order, normalised + de-duplicated.
	seen := make(map[string]struct{})
	for _, e := range entries {
		u, ok := normalizeRelayURL(e.URL)
		if !ok {
			continue
		}
		if _, dup := seen[u]; dup {
			continue
		}
		seen[u] = struct{}{}
		if kind == 10002 {
			switch {
			case e.Read && !e.Write:
				tags = append(tags, []string{"r", u, "read"})
			case e.Write && !e.Read:
				tags = append(tags, []string{"r", u, "write"})
			default: // both, or unspecified — NIP-65 omits the marker
				tags = append(tags, []string{"r", u})
			}
		} else {
			tags = append(tags, []string{tagName, u})
		}
	}

	return &nostr.Event{
		PubKey:    pubkey,
		Kind:      kind,
		CreatedAt: time.Now().Unix(),
		Content:   content,
		Tags:      tags,
	}, nil
}

// FetchRelayList returns the user's latest raw relay-list event of the given
// kind, or nil. Used by the build flow to preserve non-relay tags on republish.
// Mirrors FetchMediaServerList: index relays plus the user's cached outbox.
func (c *Client) FetchRelayList(pubkey string, kind int) *nostr.Event {
	relays := c.config.IndexRelays
	if ur, ok := c.directory.Cached(pubkey); ok {
		relays = appendUnique(relays, ur.Outbox)
	}
	return c.fetchLatestEvent(pubkey, kind, relays)
}

// InvalidateUserRelays drops a user's cached relay-role resolution so the next
// lookup re-resolves — call after the user republishes their own 10002 / 10050
// so routing picks up the change immediately.
func (c *Client) InvalidateUserRelays(pubkey string) {
	c.directory.Invalidate(pubkey)
}
