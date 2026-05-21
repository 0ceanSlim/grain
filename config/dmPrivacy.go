package config

// NIP-17 DM metadata privacy (#73), always on, no opt-out.
//
// NIP-17 says relays MAY serve kind:1059 gift wraps only to the recipient
// p-tagged on the event (enforced via NIP-42 AUTH). Without this, anyone can
// REQ {"kinds":[1059]} and read the DM social graph — who is messaging whom
// and when — even though the message content stays encrypted. grain enforces
// it unconditionally rather than behind a config knob: privacy is the default,
// and there's no legacy behavior worth preserving over leaking that graph.
//
// Only kind:1059 (gift wrap) is protected — the one kind NIP-17 names. The
// inner rumor kinds (13/14/15) should never reach a relay; if a misbehaving
// client ever publishes one it's a separate problem, not something a serving
// gate quietly papers over.
var dmProtectedKinds = map[int]bool{
	1059: true,
}

// IsDMProtectedKind reports whether events of this kind may be served only to
// the AUTHed p-tagged recipient. See the package var for rationale.
func IsDMProtectedKind(kind int) bool {
	return dmProtectedKinds[kind]
}
