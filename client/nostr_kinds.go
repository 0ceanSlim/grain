// Curated table of well-known Nostr event kinds → human-readable
// labels. Sourced from the Event Kinds section of
// github.com/nostr-protocol/nips/blob/master/README.md.
//
// Used by the admin dashboard's list-of-kinds inputs (event_purge's
// kinds_to_purge today; whitelist/blacklist kind lists later) so an
// operator sees "7 — Reaction (NIP-25)" instead of a bare integer.
//
// This is intentionally not exhaustive — only labels we can stand
// behind. An operator can still add any non-negative integer; the
// UI just shows "(no description)" for kinds not in this table.
// Update by reading the upstream README, not by inventing labels.
package client

// KindLabels maps kind → "Display Name (NIP-XX)" or
// "Display Name (deprecated)" where applicable. Keep ordered by
// kind for diffability; ranges (10000+) are NOT enumerated because
// most are addressable / replaceable buckets without specific
// well-known assignments.
var KindLabels = map[int]string{
	0:     "User Metadata (NIP-01)",
	1:     "Short Text Note (NIP-01)",
	2:     "Recommend Relay (deprecated)",
	3:     "Follow List (NIP-02)",
	4:     "Encrypted Direct Message (NIP-04, deprecated)",
	5:     "Event Deletion Request (NIP-09)",
	6:     "Repost (NIP-18)",
	7:     "Reaction (NIP-25)",
	8:     "Badge Award (NIP-58)",
	9:     "Chat Message (NIP-C7)",
	13:    "Seal (NIP-59)",
	14:    "Direct Message (NIP-17)",
	15:    "File Message (NIP-17)",
	16:    "Generic Repost (NIP-18)",
	17:    "Reaction to a Website (NIP-25)",
	20:    "Picture-First Feed (NIP-68)",
	21:    "Video Event (NIP-71)",
	22:    "Short-form Portrait Video (NIP-71)",
	40:    "Channel Creation (NIP-28)",
	41:    "Channel Metadata (NIP-28)",
	42:    "Channel Message (NIP-28)",
	43:    "Channel Hide Message (NIP-28, deprecated)",
	44:    "Channel Mute User (NIP-28, deprecated)",
	1059:  "Gift Wrap (NIP-59)",
	1063:  "File Metadata (NIP-94)",
	1311:  "Live Chat Message (NIP-53)",
	1984:  "Reporting (NIP-56)",
	1985:  "Label (NIP-32)",
	9734:  "Zap Request (NIP-57)",
	9735:  "Zap Receipt (NIP-57)",
	10000: "Mute List (NIP-51)",
	10001: "Pin List (NIP-51)",
	10002: "Relay List Metadata (NIP-65)",
	10003: "Bookmark List (NIP-51)",
	10004: "Communities List (NIP-51)",
	10005: "Public Chats List (NIP-51)",
	10006: "Blocked Relays List (NIP-51)",
	10007: "Search Relays List (NIP-51)",
	10015: "Interests List (NIP-51)",
	10030: "User Emoji List (NIP-51)",
	30000: "Follow Sets (NIP-51)",
	30002: "Relay Sets (NIP-51)",
	30003: "Bookmark Sets (NIP-51)",
	30004: "Curation Sets (NIP-51)",
	30008: "Profile Badges (NIP-58)",
	30009: "Badge Definition (NIP-58)",
	30015: "Interest Sets (NIP-51)",
	30017: "Stall (NIP-15)",
	30018: "Product (NIP-15)",
	30023: "Long-form Content (NIP-23)",
	30024: "Draft Long-form Content (NIP-23)",
	30030: "Emoji Sets (NIP-51)",
	30078: "Application-specific Data (NIP-78)",
	30311: "Live Event (NIP-53)",
	30315: "User Statuses (NIP-38)",
	30402: "Classified Listing (NIP-99)",
	30403: "Draft Classified Listing (NIP-99)",
	31922: "Date-Based Calendar Event (NIP-52)",
	31923: "Time-Based Calendar Event (NIP-52)",
	31924: "Calendar (NIP-52)",
	31925: "Calendar Event RSVP (NIP-52)",
	31989: "Handler Recommendation (NIP-89)",
	31990: "Handler Information (NIP-89)",
	34550: "Community Definition (NIP-72)",
}

// KindLabel returns the human-readable label for a known kind, or
// "" if the kind isn't in the table. Callers that want a fallback
// string should `if l := KindLabel(k); l != "" { ... }`.
func KindLabel(k int) string { return KindLabels[k] }
