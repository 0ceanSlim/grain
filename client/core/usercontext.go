package core

import (
	"context"
	"fmt"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// UserContext is the per-session handle a downstream app uses to act as one user
// on the outbox engine: their editable relay config ([SessionRelays]), an
// optional [Signer] for publishing, and the shared [Client] that owns the
// connection pool. Construct it with [Client.NewUserContext].
//
// grain's own web layer (client/api, client/session, …) is a reference consumer
// of this surface; a CLI or bot uses it directly. Read-only callers omit the
// signer.
//
// Pre-1.0 note: the network methods don't take a context.Context yet (it lands
// in a follow-up slice), and PublishDM is deferred until NIP-44 encryption is
// available. See docs/design/outbox-relay-pool.md §11.
type UserContext struct {
	client *Client
	pubkey string
	signer Signer
	relays *SessionRelays
}

// UserOption configures a [UserContext] at construction.
type UserOption func(*UserContext)

// WithSigner attaches a signer so the context can publish. Without one the
// context is read-only and [UserContext.Sign] / [UserContext.SignAndPublish]
// return an error.
func WithSigner(s Signer) UserOption {
	return func(uc *UserContext) { uc.signer = s }
}

// NewUserContext creates a [UserContext] for pubkey (64-char hex) on this
// client. Apply options such as [WithSigner] to attach a signer.
func (c *Client) NewUserContext(pubkey string, opts ...UserOption) *UserContext {
	uc := &UserContext{client: c, pubkey: pubkey, relays: newSessionRelays()}
	for _, o := range opts {
		o(uc)
	}
	return uc
}

// PublicKey returns the context user's pubkey (hex).
func (uc *UserContext) PublicKey() string { return uc.pubkey }

// Client returns the shared client that owns the connection pool.
func (uc *UserContext) Client() *Client { return uc.client }

// Signer returns the attached signer, or nil for a read-only context.
func (uc *UserContext) Signer() Signer { return uc.signer }

// Relays returns the user's editable, role-tagged session relay config.
func (uc *UserContext) Relays() *SessionRelays { return uc.relays }

// Sign signs event as this user. It requires a signer and errors if the signer's
// public key does not match the context user, so a caller can't accidentally
// sign as someone else.
func (uc *UserContext) Sign(event *nostr.Event) error {
	if uc.signer == nil {
		return fmt.Errorf("user context for %s is read-only: no signer attached", uc.pubkey)
	}
	if pk := uc.signer.PublicKey(); pk != uc.pubkey {
		return fmt.Errorf("signer pubkey %s does not match user context %s", pk, uc.pubkey)
	}
	return uc.signer.SignEvent(event)
}

// Publish routes an already-signed event under the outbox model (the author's
// outbox ∪ each p-tagged recipient's inbox; metadata also to the indexers) and
// broadcasts it, returning the per-relay results.
func (uc *UserContext) Publish(event *nostr.Event) ([]BroadcastResult, error) {
	relays := uc.client.RoutePublish(event)
	return uc.client.PublishEvent(event, relays)
}

// SignAndPublish signs event as this user and then publishes it.
func (uc *UserContext) SignAndPublish(event *nostr.Event) ([]BroadcastResult, error) {
	if err := uc.Sign(event); err != nil {
		return nil, err
	}
	return uc.Publish(event)
}

// PinFixedRelays enables the fixed-relay override — reads come from readRelays,
// writes go to writeRelays — which DISABLES the outbox model. Discouraged; for
// users who explicitly want a fixed-/single-relay client. The override is
// client-wide (see [Client.SetFixedRelays]), not scoped to this context.
func (uc *UserContext) PinFixedRelays(readRelays, writeRelays []string) {
	uc.client.SetFixedRelays(readRelays, writeRelays)
}

// ClearFixedRelays disables the override and restores outbox routing.
func (uc *UserContext) ClearFixedRelays() { uc.client.ClearFixedRelays() }

// FixedRelaysEnabled reports whether the fixed-relay override is active.
func (uc *UserContext) FixedRelaysEnabled() bool { return uc.client.FixedRelaysEnabled() }

// StreamNotes streams author's text notes (kind 1) from their outbox relays as
// each relay answers — the lazy-hydration path for a profile feed. Pass options
// like [WithLimit] to bound it. Routing honours the fixed-relay override.
func (uc *UserContext) StreamNotes(ctx context.Context, author string, opts ...StreamOption) <-chan *nostr.Event {
	relays := uc.client.RouteFetch(author)
	filter := nostr.Filter{Authors: []string{author}, Kinds: []int{1}}
	return uc.client.StreamEvents(ctx, filter, relays, opts...)
}

// FetchNotes collects author's text notes (kind 1) from their outbox relays.
// Best-effort: per-relay failures are logged, so an empty result means "none
// found" rather than a hard error. For incremental delivery use [StreamNotes].
func (uc *UserContext) FetchNotes(ctx context.Context, author string, opts ...StreamOption) []*nostr.Event {
	relays := uc.client.RouteFetch(author)
	filter := nostr.Filter{Authors: []string{author}, Kinds: []int{1}}
	return uc.client.QueryEvents(ctx, filter, relays, opts...)
}

// Reply builds a NIP-10 kind-1 reply to parent, signs it as this user, and
// publishes it under the outbox model so it reaches the parent author's inbox as
// well as the user's own audience. It returns the signed reply and the per-relay
// broadcast results. Requires a signer.
func (uc *UserContext) Reply(parent *nostr.Event, content string) (*nostr.Event, []BroadcastResult, error) {
	if parent == nil {
		return nil, nil, fmt.Errorf("reply: parent event is nil")
	}
	evt := &nostr.Event{
		Kind:      1,
		Content:   content,
		CreatedAt: time.Now().Unix(),
		Tags:      buildReplyTags(parent),
	}
	results, err := uc.SignAndPublish(evt)
	return evt, results, err
}

// buildReplyTags assembles the NIP-10 e/p tags for a reply to parent: the thread
// root and the immediate parent as marked "e" tags, plus "p" tags for everyone
// already in the thread and the parent's author, so the whole thread is notified.
func buildReplyTags(parent *nostr.Event) [][]string {
	var tags [][]string

	// Thread root: a marked "root" e-tag on the parent, else the first "e" tag
	// (positional NIP-10), else the parent itself is the root.
	root := ""
	for _, tag := range parent.Tags {
		if len(tag) >= 4 && tag[0] == "e" && tag[3] == "root" {
			root = tag[1]
			break
		}
	}
	if root == "" {
		for _, tag := range parent.Tags {
			if len(tag) >= 2 && tag[0] == "e" {
				root = tag[1]
				break
			}
		}
	}
	if root != "" && root != parent.ID {
		tags = append(tags, []string{"e", root, "", "root"})
		tags = append(tags, []string{"e", parent.ID, "", "reply"})
	} else {
		tags = append(tags, []string{"e", parent.ID, "", "root"})
	}

	// Notify everyone already in the thread, plus the parent's author.
	seen := make(map[string]struct{})
	addP := func(pk string) {
		if pk == "" {
			return
		}
		if _, ok := seen[pk]; ok {
			return
		}
		seen[pk] = struct{}{}
		tags = append(tags, []string{"p", pk})
	}
	for _, tag := range parent.Tags {
		if len(tag) >= 2 && tag[0] == "p" {
			addP(tag[1])
		}
	}
	addP(parent.PubKey)

	return tags
}
