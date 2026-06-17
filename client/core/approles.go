package core

// Locally-configured ("app") relay roles are session preferences a downstream
// app edits — seeded from the operator's config, overridable per session, and
// NOT published as Nostr lists (unlike the per-target/self-only event-derived
// roles). Today the routing-affecting ones are RoleIndexer (resolution seeds)
// and RoleBroadcast (writes mirror there); RoleLocal and RoleTrusted are stored
// but inert until their wiring lands (Local routing preference; Trusted needs
// NIP-42 AUTH, see #101).

// AppRelays returns the session relays for a locally-configured role. RoleIndexer
// falls back to the configured index relays when the user hasn't overridden it;
// the other roles return nil when unset.
func (c *Client) AppRelays(role Role) []string {
	c.appRelaysMu.Lock()
	v, ok := c.appRelays[role]
	c.appRelaysMu.Unlock()
	if ok {
		return append([]string(nil), v...)
	}
	if role == RoleIndexer {
		return append([]string(nil), c.config.IndexRelays...)
	}
	return nil
}

// SetAppRelays sets (or clears, when urls is empty) the session override for a
// locally-configured role. Clearing RoleIndexer restores the configured default.
func (c *Client) SetAppRelays(role Role, urls []string) {
	urls = normalizeRelayURLs(urls)
	c.appRelaysMu.Lock()
	if len(urls) == 0 {
		delete(c.appRelays, role)
	} else {
		c.appRelays[role] = urls
	}
	c.appRelaysMu.Unlock()
}

// indexRelays is the effective index-relay set for routing and resolution: the
// session's RoleIndexer override if set, else the configured defaults. Used in
// place of c.config.IndexRelays everywhere routing/discovery reads the seeds.
func (c *Client) indexRelays() []string {
	return c.AppRelays(RoleIndexer)
}
