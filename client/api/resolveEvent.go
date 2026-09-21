package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/0ceanslim/grain/client/connection"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ResolveEventHandler resolves a SINGLE event by id, escalating across relay
// sets until it turns up: the local relay (grain itself) → the author's outbox
// relays → explicit relay hints → the wider discovered set. It returns as soon
// as the event is found, or 404 after every tier misses.
//
// Why this exists separately from QueryEventsHandler: that handler fans out to
// the connected *index* pool with no relay hints — right for metadata and relay
// lists, wrong for arbitrary notes. The note you click in the live feed lives on
// grain's OWN relay, which isn't in the connected pool, so tier 1 queries it
// directly; and when it isn't local, the author's pubkey lets us route to their
// outbox (the outbox model) instead of guessing.
//
// Query: GET /api/v1/events/{id}?author=<hex>&relays=<comma-separated urls>
//
// @Summary      Resolve a single event by id (outbox-aware)
// @Description  Escalating lookup: local relay → author outbox → relay hints → discovered set; returns on first hit.
// @Tags         client
// @Produce      json
// @Param        id      path   string  true   "Event ID (hex)"
// @Param        author  query  string  false  "Author pubkey (hex) — enables outbox routing"
// @Param        relays  query  string  false  "Comma-separated relay hints to try"
// @Success      200  {object}  map[string]nostr.Event
// @Failure      404  {object}  map[string]string  "Event not found"
// @Router       /api/v1/events/{id} [get]
func ResolveEventHandler(w http.ResponseWriter, r *http.Request) {
	// The route pattern carries no method (avoids a ServeMux conflict with the
	// literal /events/query, /events/publish routes), so enforce GET here.
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "event id required", http.StatusBadRequest)
		return
	}
	author := strings.TrimSpace(r.URL.Query().Get("author"))
	hints := splitRelayList(r.URL.Query().Get("relays"))

	cc := connection.GetCoreClient()
	if cc == nil {
		http.Error(w, "Client not available", http.StatusInternalServerError)
		return
	}
	_ = connection.EnsureRelayConnections()

	limit := 1
	filter := nostr.Filter{IDs: []string{id}, Limit: &limit}

	// Try one relay set; return the id-matching event if any relay in it has it.
	try := func(relays []string, timeout time.Duration) *nostr.Event {
		relays = dedupeNonEmpty(relays)
		if len(relays) == 0 {
			return nil
		}
		for _, ev := range cc.FetchEvents(r.Context(), []nostr.Filter{filter}, relays, 1, timeout) {
			if ev != nil && ev.ID == id {
				return ev
			}
		}
		return nil
	}

	var found *nostr.Event

	// Tier 1 — the local relay (grain itself, absent from the connected pool)
	// plus whatever the pool is already connected to.
	found = try(append([]string{selfRelayURL(r)}, cc.GetConnectedRelays()...), 3*time.Second)

	// Tier 2 — the author's outbox relays (resolve their NIP-65 list first).
	if found == nil && author != "" {
		cc.WarmRelays(author)
		found = try(cc.RouteFetch(author), 4*time.Second)
	}

	// Tier 3 — explicit relay hints (advanced input / nevent TLV).
	if found == nil && len(hints) > 0 {
		found = try(hints, 4*time.Second)
	}

	// Tier 4 — widen to the discovered (NIP-66) set as a last resort.
	if found == nil {
		found = try(cc.DiscoveredRelayURLs(), 5*time.Second)
	}

	w.Header().Set("Content-Type", "application/json")
	if found == nil {
		log.ClientAPI().Debug("Event not resolved after all tiers",
			"id", id, "author", author, "hints", len(hints))
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "event not found"})
		return
	}
	log.ClientAPI().Debug("Event resolved", "id", id, "kind", found.Kind)
	if err := json.NewEncoder(w).Encode(map[string]*nostr.Event{"event": found}); err != nil {
		log.ClientAPI().Error("Failed to encode resolved event", "error", err)
	}
}

// selfRelayURL builds the ws(s):// URL of grain's own relay from the request, so
// the resolver can query the local store (which isn't in the client's pool).
func selfRelayURL(r *http.Request) string {
	scheme := "ws://"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "wss://"
	}
	return scheme + r.Host
}

// splitRelayList parses the comma-separated ?relays= hint into normalized URLs.
func splitRelayList(csv string) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, "ws://") && !strings.HasPrefix(p, "wss://") {
			p = "wss://" + p
		}
		out = append(out, p)
	}
	return out
}

// dedupeNonEmpty drops empties and duplicates while preserving order.
func dedupeNonEmpty(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
