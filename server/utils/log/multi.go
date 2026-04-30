package log

import (
	"context"
	"errors"
	"log/slog"
)

// multiHandler fans out slog records to multiple inner handlers. Used to
// run a file handler and a stdout handler side-by-side so deployments
// can keep the canonical log file *and* expose live output to
// `docker logs` (or `journalctl`, or a tee'd terminal). Errors from
// individual handlers are joined so a stdout write failure does not
// suppress the file write or vice versa.
type multiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler returns a slog.Handler that fans Handle/WithAttrs/
// WithGroup calls out to all provided handlers. nil entries are skipped.
func NewMultiHandler(handlers ...slog.Handler) slog.Handler {
	filtered := make([]slog.Handler, 0, len(handlers))
	for _, h := range handlers {
		if h != nil {
			filtered = append(filtered, h)
		}
	}
	return &multiHandler{handlers: filtered}
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range m.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		if err := h.Handle(ctx, r.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: next}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: next}
}
