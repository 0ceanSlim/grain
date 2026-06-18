package core

import "strings"

// Role identifies the function a relay serves for a user under the outbox model.
// Roles fall into three classes (see docs/design/outbox-relay-pool.md §3):
//
//   - Per-target, event-derived: [RoleOutbox], [RoleInbox], [RoleDMInbox] —
//     resolved from the *target* user's published NIP-65 / NIP-17 lists.
//   - Self-only, event-derived: [RoleSearch], [RoleBlocked], [RoleFavorite],
//     [RolePrivateHome] — the logged-in user's own NIP-51 / NIP-37 lists.
//   - Locally configured: [RoleIndexer], [RoleBroadcast], [RoleProxy],
//     [RoleLocal], [RoleTrusted] — app config, applied across every user you
//     interact with.
//
// A relay may hold several roles at once (a relay can be both your outbox and
// your inbox), so Role is a bitmask: combine with | and test with [Role.Has].
type Role uint16

const (
	RoleOutbox      Role = 1 << iota // NIP-65 write (10002): you publish here; others fetch your notes here
	RoleInbox                        // NIP-65 read (10002): replies, mentions, and zaps reach you here
	RoleDMInbox                      // NIP-17 (10050): encrypted direct messages are delivered here
	RoleSearch                       // NIP-51 (10007): relays queried for NIP-50 search
	RoleBlocked                      // NIP-51 (10006): never dialed
	RoleFavorite                     // NIP-51 (10012): surfaced in UI; no routing effect
	RolePrivateHome                  // NIP-37 (10013, NIP-44 encrypted): relays for private content
	RoleIndexer                      // local config: seeds for resolving anyone's metadata / relay lists
	RoleBroadcast                    // local config: fan-out relays ("event blasters"); writes mirror here
	RoleProxy                        // local config: aggregator; when set, short-circuits outbox routing
	RoleLocal                        // local config: same-device / LAN relays, preferred for latency
	RoleTrusted                      // local config: relays the client will sign NIP-42 AUTH challenges for
)

// roleNames pairs each single-bit role with a stable lowercase name, in bit
// order, for [Role.String].
var roleNames = []struct {
	role Role
	name string
}{
	{RoleOutbox, "outbox"},
	{RoleInbox, "inbox"},
	{RoleDMInbox, "dm-inbox"},
	{RoleSearch, "search"},
	{RoleBlocked, "blocked"},
	{RoleFavorite, "favorite"},
	{RolePrivateHome, "private-home"},
	{RoleIndexer, "indexer"},
	{RoleBroadcast, "broadcast"},
	{RoleProxy, "proxy"},
	{RoleLocal, "local"},
	{RoleTrusted, "trusted"},
}

// Has reports whether r includes every bit of x (and x is non-zero). For a
// single-bit role this is a plain membership test; for a combination it checks
// that all of x's roles are present.
func (r Role) Has(x Role) bool { return x != 0 && r&x == x }

// String renders the set roles as a "|"-joined list (e.g. "outbox|inbox"), or
// "none" when no bits are set. Intended for logs and inspection, not parsing.
func (r Role) String() string {
	if r == 0 {
		return "none"
	}
	parts := make([]string, 0, len(roleNames))
	for _, rn := range roleNames {
		if r&rn.role != 0 {
			parts = append(parts, rn.name)
		}
	}
	return strings.Join(parts, "|")
}

// RoleForListKind maps a replaceable relay-list event kind to the role(s) its
// `relay`/`r` entries carry — e.g. 10002 → RoleOutbox|RoleInbox (the read/write
// split is per-entry markers), 10050 → RoleDMInbox. ok is false for kinds that
// are not relay lists.
func RoleForListKind(kind int) (role Role, ok bool) {
	switch kind {
	case 10002:
		return RoleOutbox | RoleInbox, true
	case 10050:
		return RoleDMInbox, true
	case 10006:
		return RoleBlocked, true
	case 10007:
		return RoleSearch, true
	case 10012:
		return RoleFavorite, true
	case 10013:
		return RolePrivateHome, true
	}
	return 0, false
}

// ForRole returns the resolved per-target relays carrying the given role:
// [RoleOutbox], [RoleInbox], or [RoleDMInbox]. Any other role returns nil — the
// rest are self-only or locally configured, not part of a per-target resolution.
func (ur *UserRelays) ForRole(role Role) []string {
	if ur == nil {
		return nil
	}
	switch role {
	case RoleOutbox:
		return ur.Outbox
	case RoleInbox:
		return ur.Inbox
	case RoleDMInbox:
		return ur.DMInbox
	}
	return nil
}

// RouteOp names a read-routing intent so a caller can inspect which relays an
// operation would use, via [Client.Route], without performing it.
type RouteOp int

const (
	OpFetchNotes    RouteOp = iota // a user's authored events → their outbox (NIP-65 write)
	OpFetchMetadata                // profile / relay lists → indexers (+ cached outbox)
)

// Route returns the relays a read operation against target would use under the
// current routing, honouring the fixed-relay override. It is a thin, inspectable
// facade over [Client.RouteFetch] / [Client.RouteMetadata].
//
// Publishing is intentionally not covered here: it routes per-event (author
// outbox ∪ each recipient's inbox), so use [Client.RoutePublish] with the event.
func (c *Client) Route(op RouteOp, target string) []string {
	switch op {
	case OpFetchMetadata:
		return c.RouteMetadata(target)
	default:
		return c.RouteFetch(target)
	}
}
