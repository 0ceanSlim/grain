package core

import nostr "github.com/0ceanslim/grain/server/types"

// DefaultClientTagName is grain's client-tag value when none is configured. The
// "on by default" semantics live in the resolving handler + config loader; this
// is just the fallback name.
const DefaultClientTagName = "grain"

// applyClientTag strips every foreign `client` tag from tags and, when enabled,
// appends grain's own ["client", name]. Always strips; conditionally adds.
// Returns a new slice — the input is not mutated.
func applyClientTag(tags [][]string, enabled bool, name string) [][]string {
	out := make([][]string, 0, len(tags)+1)
	for _, t := range tags {
		if len(t) >= 1 && t[0] == "client" {
			continue // drop foreign client tags (e.g. a carried-over client:amethyst)
		}
		out = append(out, t)
	}
	if enabled {
		if name == "" {
			name = DefaultClientTagName
		}
		out = append(out, []string{"client", name})
	}
	return out
}

// ApplyClientTag rewrites an event's client tag in place per grain's policy:
// strip any foreign `client` tag always, add grain's own when enabled. Safe on a
// nil event. Build handlers call this after assembling an event grain authors.
func ApplyClientTag(event *nostr.Event, enabled bool, name string) {
	if event == nil {
		return
	}
	event.Tags = applyClientTag(event.Tags, enabled, name)
}
