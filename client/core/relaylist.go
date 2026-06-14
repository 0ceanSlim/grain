package core

import (
	"fmt"
	"sync"
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

// ParseNIP65Entries parses a kind-10002 event's `r` tags into relay entries with
// read/write flags. An unmarked entry (`["r", url]`) is both read and write;
// `["r", url, "read"]` / `["r", url, "write"]` set one side. URLs are normalised;
// a relay listed twice has its flags OR-ed together.
func ParseNIP65Entries(event *nostr.Event) []RelayListEntry {
	var out []RelayListEntry
	seen := make(map[string]int)
	for _, t := range event.Tags {
		if len(t) < 2 || t[0] != "r" || t[1] == "" {
			continue
		}
		u, ok := normalizeRelayURL(t[1])
		if !ok {
			continue
		}
		read, write := true, true
		if len(t) >= 3 {
			switch t[2] {
			case "read":
				read, write = true, false
			case "write":
				read, write = false, true
			}
		}
		if idx, dup := seen[u]; dup {
			out[idx].Read = out[idx].Read || read
			out[idx].Write = out[idx].Write || write
			continue
		}
		seen[u] = len(out)
		out = append(out, RelayListEntry{URL: u, Read: read, Write: write})
	}
	return out
}

// ParseRelayTagURLs extracts the `relay` tag URLs from a NIP-17 / NIP-51
// relay-list event (kinds 10050 / 10006 / 10007 / 10012), normalised and
// de-duplicated, in order.
func ParseRelayTagURLs(event *nostr.Event) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, t := range event.Tags {
		if len(t) >= 2 && t[0] == "relay" && t[1] != "" {
			u, ok := normalizeRelayURL(t[1])
			if !ok {
				continue
			}
			if _, dup := seen[u]; dup {
				continue
			}
			seen[u] = struct{}{}
			out = append(out, u)
		}
	}
	return out
}

// UserRelayLists is a user's resolved relay lists across the kinds the relay
// manager edits. NIP65 carries read/write flags; the rest are plain URL lists.
type UserRelayLists struct {
	NIP65     []RelayListEntry `json:"nip65"`     // 10002
	DM        []string         `json:"dm"`        // 10050
	Blocked   []string         `json:"blocked"`   // 10006
	Search    []string         `json:"search"`    // 10007
	Favorites []string         `json:"favorites"` // 10012
	// Encrypted flags NIP-51 lists whose event carried NIP-44 private content —
	// entries grain can't read yet (#100); only the public entries are listed.
	Encrypted EncryptedFlags `json:"encrypted"`
}

// EncryptedFlags reports which NIP-51 lists have private (encrypted) entries.
type EncryptedFlags struct {
	Blocked   bool `json:"blocked"`
	Search    bool `json:"search"`
	Favorites bool `json:"favorites"`
}

// FetchUserRelayLists resolves a user's relay lists for every kind the relay
// manager shows, fetching the kinds concurrently. Each goroutine writes a
// distinct field, so no locking is needed.
//
// A user's own NIP-51/17 lists live on their own relays, which a cold page load
// may not have cached — so the relay set is the index relays PLUS the user's
// resolved read+write relays, giving the lists a real chance to be found.
func (c *Client) FetchUserRelayLists(pubkey string) *UserRelayLists {
	relays := append([]string(nil), c.config.IndexRelays...)
	ur := c.ResolveRelays(pubkey) // blocking resolve — own user is cached post-login
	relays = appendUnique(relays, ur.Outbox)
	relays = appendUnique(relays, ur.Inbox)

	out := &UserRelayLists{}
	var wg sync.WaitGroup
	fetch := func(kind int, assign func(*nostr.Event)) {
		defer wg.Done()
		if ev := c.fetchLatestEvent(pubkey, kind, relays); ev != nil {
			assign(ev)
		}
	}
	wg.Add(5)
	go fetch(10002, func(ev *nostr.Event) { out.NIP65 = ParseNIP65Entries(ev) })
	go fetch(10050, func(ev *nostr.Event) { out.DM = ParseRelayTagURLs(ev) })
	go fetch(10006, func(ev *nostr.Event) {
		out.Blocked = ParseRelayTagURLs(ev)
		out.Encrypted.Blocked = ev.Content != ""
	})
	go fetch(10007, func(ev *nostr.Event) {
		out.Search = ParseRelayTagURLs(ev)
		out.Encrypted.Search = ev.Content != ""
	})
	go fetch(10012, func(ev *nostr.Event) {
		out.Favorites = ParseRelayTagURLs(ev)
		out.Encrypted.Favorites = ev.Content != ""
	})
	wg.Wait()
	return out
}
