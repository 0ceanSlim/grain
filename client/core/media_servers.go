package core

// Media-server protocol kinds. Blossom is nostr-native (blobs addressed by
// sha256, signed kind-24242 auth); NIP-96 is traditional HTTP storage authed
// with NIP-98. grain prefers Blossom and treats NIP-96 as a legacy fallback.
const (
	MediaKindBlossom = "blossom" // BUD-03 kind 10063; BUD-02 /upload, BUD-04 /mirror
	MediaKindNIP96   = "nip96"   // kind 10096; NIP-96 multipart upload + NIP-98 auth
)

// MediaServerInfo is static metadata about a media server grain knows: what
// protocol it speaks, what it costs, how long it keeps blobs, and whether it
// accepts BUD-04 mirror requests. It drives the free/paid + retention chips in
// the settings UI and decides which servers can be offered as mirror targets.
type MediaServerInfo struct {
	URL       string `json:"url"`       // normalised base URL
	Kind      string `json:"kind"`      // MediaKindBlossom | MediaKindNIP96
	Name      string `json:"name"`      // display name, e.g. "Happy Tavern"
	Cost      string `json:"cost"`      // "free" | "paid"
	Retention string `json:"retention"` // "permanent" | "ephemeral"
	Mirror    bool   `json:"mirror"`    // accepts BUD-04 /mirror (Blossom only)
	Note      string `json:"note"`      // short blurb shown under the entry
	CTA       string `json:"cta"`       // optional signup / pricing link
}

// suggestedMediaServers is grain's curated quick-add list for users with no
// media servers yet. Blossom is preferred; NIP-96 is the legacy fallback.
// Neutral free options lead; the Happy Tavern offer is included rather than
// pushed (the settings UI keeps it unobtrusive).
//
// Mirror support: the Happy Tavern servers are confirmed to accept BUD-04
// /mirror. The public Blossom servers are marked best-effort — BUD-04 is widely
// supported, and a server that turns out not to accept a mirror just surfaces a
// per-server failure in the upload toast (no data loss), so this stays
// permissive rather than gating. Verified at runtime when used as a target.
var suggestedMediaServers = []MediaServerInfo{
	// Blossom (preferred)
	{
		URL: "https://blossom.band", Kind: MediaKindBlossom, Name: "blossom.band",
		Cost: "free", Retention: "permanent", Mirror: true,
		Note: "Free · ~20 MB/file · runs on nostr.build infra",
	},
	{
		URL: "https://0x0.happytavern.co", Kind: MediaKindBlossom, Name: "Happy Tavern (free)",
		Cost: "free", Retention: "ephemeral", Mirror: true,
		Note: "Free · auto-pruned by size/age — pair as a fast primary with a permanent mirror",
	},
	{
		URL: "https://blossom.happytavern.co", Kind: MediaKindBlossom, Name: "Happy Tavern (permanent)",
		Cost: "paid", Retention: "permanent", Mirror: true,
		Note: "Tavern membership · 10k sats one-time · 100 MB/file · 8 GB · kept forever",
		CTA:  "https://happytavern.co/nostr-verified",
	},
	{
		URL: "https://blossom.primal.net", Kind: MediaKindBlossom, Name: "Primal",
		Cost: "free", Retention: "permanent", Mirror: true,
		Note: "Free · reliable, widely-used default",
	},
	// NIP-96 (legacy fallback)
	{
		URL: "https://nostr.build", Kind: MediaKindNIP96, Name: "nostr.build",
		Cost: "free", Retention: "permanent", Mirror: false,
		Note: "Legacy HTTP storage · free tier + paid upgrades",
	},
	{
		URL: "https://nostrcheck.me", Kind: MediaKindNIP96, Name: "nostrcheck.me",
		Cost: "free", Retention: "permanent", Mirror: false,
		Note: "Legacy HTTP storage · open source, self-hostable",
	},
}

// SuggestedMediaServers returns a copy of grain's curated quick-add suggestions
// (Blossom preferred, NIP-96 legacy fallback).
func SuggestedMediaServers() []MediaServerInfo {
	out := make([]MediaServerInfo, len(suggestedMediaServers))
	copy(out, suggestedMediaServers)
	return out
}

// LookupMediaServerInfo returns grain's static capability metadata for a server
// URL if it knows it, matching on the normalised URL. The boolean reports a
// hit; unknown servers (a user's own additions) simply carry no chips.
func LookupMediaServerInfo(rawURL string) (MediaServerInfo, bool) {
	norm, ok := normalizeMediaURL(rawURL)
	if !ok {
		return MediaServerInfo{}, false
	}
	for _, info := range suggestedMediaServers {
		if info.URL == norm {
			return info, true
		}
	}
	return MediaServerInfo{}, false
}
