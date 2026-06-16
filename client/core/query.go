package core

import (
	"context"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// streamConfig is the resolved set of [StreamOption]s.
type streamConfig struct {
	limit   int           // stop after this many events (0 = unlimited)
	timeout time.Duration // overall cap before closing
	live    bool          // keep streaming after end-of-stored-events
}

// StreamOption configures [Client.StreamEvents] / [Client.QueryEvents].
type StreamOption func(*streamConfig)

// WithLimit closes the stream after n events have been delivered (0 = no limit).
func WithLimit(n int) StreamOption { return func(c *streamConfig) { c.limit = n } }

// WithTimeout caps how long the stream runs before closing (default 10s). The
// stream also ends when every relay reports end-of-stored-events (unless
// [WithLive]), the limit is reached, or the context is cancelled.
func WithTimeout(d time.Duration) StreamOption { return func(c *streamConfig) { c.timeout = d } }

// WithLive keeps the stream open past every relay's end-of-stored-events, so it
// keeps delivering newly published events until the context or timeout ends it.
// Default is a bounded fetch that closes once all stored events are in.
func WithLive() StreamOption { return func(c *streamConfig) { c.live = true } }

// StreamEvents subscribes to filter across relays and streams matching events on
// the returned channel as each relay answers, de-duplicated by event id. The
// channel closes when every relay has signalled end-of-stored-events (a bounded
// fetch; see [WithLive]), the [WithLimit] count is reached, ctx is cancelled, or
// the [WithTimeout] elapses — whichever comes first.
//
// This is the general, multi-relay form of a live feed: point it at a single
// relay (e.g. grain's own) for a single-source stream, or at a user's outbox set
// for an outbox feed. Per-relay failures are logged, not fatal. The caller must
// drain the channel (or cancel ctx) so the underlying subscription is released.
func (c *Client) StreamEvents(ctx context.Context, filter nostr.Filter, relays []string, opts ...StreamOption) <-chan *nostr.Event {
	cfg := streamConfig{timeout: 10 * time.Second}
	for _, o := range opts {
		o(&cfg)
	}

	out := make(chan *nostr.Event)
	go func() {
		defer close(out)
		if len(relays) == 0 {
			return
		}
		sub, err := c.Subscribe(ctx, []nostr.Filter{filter}, relays)
		if err != nil {
			log.ClientCore().Debug("StreamEvents subscribe failed", "error", err)
			return
		}
		defer sub.Close()

		var deadline <-chan time.Time
		if cfg.timeout > 0 {
			deadline = time.After(cfg.timeout)
		}
		seen := make(map[string]struct{})
		eose := make(map[string]bool)
		sent := 0

		for {
			select {
			case <-ctx.Done():
				return
			case <-deadline:
				return
			case ev := <-sub.Events:
				if ev == nil || ev.ID == "" {
					continue
				}
				if _, dup := seen[ev.ID]; dup {
					continue // same event from another relay
				}
				seen[ev.ID] = struct{}{}
				select {
				case out <- ev:
				case <-ctx.Done():
					return
				}
				if sent++; cfg.limit > 0 && sent >= cfg.limit {
					return
				}
			case relayURL := <-sub.EOSE:
				eose[relayURL] = true
				if !cfg.live && len(eose) >= len(relays) {
					return // every relay has sent all its stored events
				}
			case err := <-sub.Errors:
				log.ClientCore().Debug("StreamEvents relay error", "error", err)
			}
		}
	}()

	return out
}

// QueryEvents runs [Client.StreamEvents] and collects every event it yields, in
// arrival order. A blocking convenience for callers that don't need incremental
// delivery; pass [WithLimit] / [WithTimeout] to bound it.
func (c *Client) QueryEvents(ctx context.Context, filter nostr.Filter, relays []string, opts ...StreamOption) []*nostr.Event {
	var out []*nostr.Event
	for ev := range c.StreamEvents(ctx, filter, relays, opts...) {
		out = append(out, ev)
	}
	return out
}
