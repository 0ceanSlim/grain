package core

import (
	"net/url"
	"strings"
	"sync"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// MediaServers holds a user's published media-server lists: Blossom servers
// (NIP-B7 / BUD-03 kind 10063) and legacy NIP-96 HTTP servers (kind 10096).
// Both are `["server", <url>]` tag lists in preference order — the first entry
// is the user's primary, the rest are mirrors / fallbacks. Resolved from the
// network and cached with a TTL, mirroring the relay directory.
type MediaServers struct {
	Blossom   []string  // kind 10063 — Blossom servers, primary first
	NIP96     []string  // kind 10096 — legacy NIP-96 HTTP servers, primary first
	FetchedAt time.Time // when this was resolved
	Negative  bool      // user has published neither list — cached briefly
}

// HasAny reports whether the user has any media server configured at all. The
// upload flow uses this to decide between "open the picker" and "prompt the
// user to set some up".
func (m *MediaServers) HasAny() bool {
	return m != nil && (len(m.Blossom) > 0 || len(m.NIP96) > 0)
}

// MediaDirectory caches per-user media-server resolutions with a TTL, collapsing
// concurrent lookups for the same pubkey into a single network fetch. It is
// structurally identical to RelayDirectory but kept separate: media servers are
// a distinct concern from relays and resolve from different event kinds.
type MediaDirectory struct {
	mu       sync.Mutex
	entries  map[string]*MediaServers
	inflight map[string]chan struct{}
	ttl      time.Duration
	negTTL   time.Duration
	resolve  func(pubkey string) *MediaServers
}

func newMediaDirectory(ttl, negTTL time.Duration, resolve func(string) *MediaServers) *MediaDirectory {
	if ttl <= 0 {
		ttl = time.Hour
	}
	if negTTL <= 0 {
		negTTL = time.Minute
	}
	return &MediaDirectory{
		entries:  make(map[string]*MediaServers),
		inflight: make(map[string]chan struct{}),
		ttl:      ttl,
		negTTL:   negTTL,
		resolve:  resolve,
	}
}

// fresh reports whether a cached entry is still within its TTL. A "no list
// published" result uses the shorter negative TTL so a user who sets up media
// servers is picked up soon instead of looking server-less for an hour.
func (d *MediaDirectory) fresh(m *MediaServers) bool {
	if m == nil {
		return false
	}
	ttl := d.ttl
	if m.Negative {
		ttl = d.negTTL
	}
	return time.Since(m.FetchedAt) < ttl
}

// Lookup returns the user's media servers, resolving and caching on a miss or
// stale entry. Concurrent lookups for the same pubkey share one resolution.
func (d *MediaDirectory) Lookup(pubkey string) *MediaServers {
	for {
		d.mu.Lock()
		if m, ok := d.entries[pubkey]; ok && d.fresh(m) {
			d.mu.Unlock()
			return m
		}
		if ch, ok := d.inflight[pubkey]; ok {
			d.mu.Unlock()
			<-ch
			continue
		}
		ch := make(chan struct{})
		d.inflight[pubkey] = ch
		d.mu.Unlock()

		m := d.resolve(pubkey)
		if m == nil {
			m = &MediaServers{Negative: true}
		}
		// Stamp the fetch time here, not in the resolver, so a resolver that
		// forgets can't silently disable caching (entry never reads as fresh).
		m.FetchedAt = time.Now()

		d.mu.Lock()
		d.entries[pubkey] = m
		delete(d.inflight, pubkey)
		close(ch)
		d.mu.Unlock()
		return m
	}
}

// Invalidate drops a cached entry so the next Lookup re-resolves — call after
// the user publishes an updated list, so the change shows immediately.
func (d *MediaDirectory) Invalidate(pubkey string) {
	d.mu.Lock()
	delete(d.entries, pubkey)
	d.mu.Unlock()
}

// ResolveMediaServers returns a user's Blossom (kind 10063) and NIP-96
// (kind 10096) media-server lists, resolved from their published events and
// cached with a TTL. Safe for concurrent use; the logged-in user is just
// another pubkey.
func (c *Client) ResolveMediaServers(pubkey string) *MediaServers {
	return c.mediaDir.Lookup(pubkey)
}

// InvalidateMediaServers drops a user's cached media-server lists so the next
// resolve re-fetches — used after the user publishes an updated list.
func (c *Client) InvalidateMediaServers(pubkey string) {
	c.mediaDir.Invalidate(pubkey)
}

// fetchUserMediaServersFromNetwork is the MediaDirectory's resolver: it queries
// the index relays AND the user's own outbox (when already cached) for the
// user's latest kind 10063 (Blossom) and 10096 (NIP-96) events concurrently and
// maps them to server lists.
//
// The outbox matters here more than for profiles: media-server lists are often
// published only to the user's own write relays, not the metadata indexers, so
// querying indexers alone can miss a list the user really has. The cached
// outbox is included without a blocking resolve (same rule as RouteMetadata);
// for the logged-in user it is warmed by the mailbox lookup done at login.
func (c *Client) fetchUserMediaServersFromNetwork(pubkey string) *MediaServers {
	relays := c.config.IndexRelays
	if len(relays) == 0 {
		relays = c.GetConnectedRelays()
	}
	if ur, ok := c.directory.Cached(pubkey); ok {
		relays = appendUnique(relays, ur.Outbox)
	}

	var (
		blossom *nostr.Event
		nip96   *nostr.Event
		wg      sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		blossom = c.fetchLatestEvent(pubkey, 10063, relays)
	}()
	go func() {
		defer wg.Done()
		nip96 = c.fetchLatestEvent(pubkey, 10096, relays)
	}()
	wg.Wait()

	m := &MediaServers{FetchedAt: time.Now()}
	if blossom != nil {
		m.Blossom = parseServerListEvent(blossom)
	}
	if nip96 != nil {
		m.NIP96 = parseServerListEvent(nip96)
	}
	m.Negative = len(m.Blossom) == 0 && len(m.NIP96) == 0

	log.ClientCore().Debug("Resolved user media servers", "pubkey", pubkey,
		"blossom", len(m.Blossom), "nip96", len(m.NIP96), "negative", m.Negative)
	return m
}

// parseServerListEvent extracts the `["server", <url>]` tag values from a
// media-server list event (Blossom kind 10063 / NIP-96 kind 10096), preserving
// order (primary first) and dropping duplicates. URLs are normalised so
// near-duplicate entries collapse.
func parseServerListEvent(event *nostr.Event) []string {
	var out []string
	seen := make(map[string]struct{})
	for _, tag := range event.Tags {
		if len(tag) >= 2 && tag[0] == "server" && tag[1] != "" {
			u, ok := normalizeMediaURL(tag[1])
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

// normalizeMediaURL canonicalises a media-server base URL: it defaults to
// https, lowercases the host, and strips a trailing slash, so near-duplicate
// entries collapse. Unlike relay URLs these are HTTP(S), not ws/wss. Returns
// false for input that can't be made into a valid absolute http(s) URL.
func normalizeMediaURL(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", false
	}
	switch u.Scheme {
	case "http", "https":
	default:
		return "", false
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	u.Fragment = ""
	u.RawQuery = ""
	return u.String(), true
}
