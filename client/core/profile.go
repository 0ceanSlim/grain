package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// AssembleProfileEvent builds an UNSIGNED kind-0 (profile metadata) event from
// an existing one by applying edits. It is deliberately conservative about not
// losing data:
//
//   - Every existing content field is preserved; only edited fields are
//     overwritten. The content JSON stays the interoperable source of truth
//     that all clients read.
//   - Every existing tag is preserved; for each edited field a [key, value] tag
//     is upserted — replacing a prior tag with the same key rather than
//     duplicating — so tag-aware clients can read the field too (dual-write).
//
// The returned event has no ID or Sig: the caller signs it. Returns an error if
// the existing content isn't valid JSON, rather than silently clobbering a
// profile we can't safely parse.
func AssembleProfileEvent(existing *nostr.Event, edits map[string]string) (*nostr.Event, error) {
	pubkey := ""
	var existingTags [][]string
	content := map[string]json.RawMessage{}

	if existing != nil {
		pubkey = existing.PubKey
		existingTags = existing.Tags
		if existing.Content != "" {
			if err := json.Unmarshal([]byte(existing.Content), &content); err != nil {
				return nil, fmt.Errorf("existing kind-0 content is not valid JSON; refusing to overwrite: %w", err)
			}
		}
	}

	// Apply edits to the content map, preserving every other field.
	for k, v := range edits {
		raw, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("encode field %q: %w", k, err)
		}
		content[k] = raw
	}
	newContent, err := json.Marshal(content) // encoding/json marshals map keys sorted
	if err != nil {
		return nil, err
	}

	// Preserve existing tags except the ones we're about to upsert.
	edited := make(map[string]bool, len(edits))
	for k := range edits {
		edited[k] = true
	}
	tags := make([][]string, 0, len(existingTags)+len(edits))
	for _, t := range existingTags {
		if len(t) >= 1 && edited[t[0]] {
			continue // replaced below, not duplicated
		}
		tags = append(tags, t)
	}
	// Append the dual-write tags in a stable (sorted) order.
	keys := make([]string, 0, len(edits))
	for k := range edits {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		tags = append(tags, []string{k, edits[k]})
	}

	return &nostr.Event{
		PubKey:    pubkey,
		Kind:      0,
		CreatedAt: time.Now().Unix(),
		Content:   string(newContent),
		Tags:      tags,
	}, nil
}
