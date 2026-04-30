package log

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// captureHandler is a tiny slog.Handler used in tests to record every
// record it receives. Mirrors the minimum slog.Handler surface the
// multiHandler needs to fan out.
type captureHandler struct {
	level slog.Level
	buf   *bytes.Buffer
	attrs []slog.Attr
	err   error
}

func (c *captureHandler) Enabled(_ context.Context, l slog.Level) bool { return l >= c.level }

func (c *captureHandler) Handle(_ context.Context, r slog.Record) error {
	if c.err != nil {
		return c.err
	}
	c.buf.WriteString(r.Message)
	for _, a := range c.attrs {
		c.buf.WriteString(" ")
		c.buf.WriteString(a.Key)
		c.buf.WriteString("=")
		c.buf.WriteString(a.Value.String())
	}
	r.Attrs(func(a slog.Attr) bool {
		c.buf.WriteString(" ")
		c.buf.WriteString(a.Key)
		c.buf.WriteString("=")
		c.buf.WriteString(a.Value.String())
		return true
	})
	c.buf.WriteString("\n")
	return nil
}

func (c *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &captureHandler{level: c.level, buf: c.buf, attrs: append(c.attrs, attrs...), err: c.err}
}

func (c *captureHandler) WithGroup(_ string) slog.Handler { return c }

func TestMultiHandler_FansOutToAll(t *testing.T) {
	a := &captureHandler{level: slog.LevelDebug, buf: &bytes.Buffer{}}
	b := &captureHandler{level: slog.LevelDebug, buf: &bytes.Buffer{}}
	h := NewMultiHandler(a, b)

	logger := slog.New(h)
	logger.Info("hello", "k", "v")

	if !strings.Contains(a.buf.String(), "hello") || !strings.Contains(a.buf.String(), "k=v") {
		t.Errorf("handler a missed record: %q", a.buf.String())
	}
	if !strings.Contains(b.buf.String(), "hello") || !strings.Contains(b.buf.String(), "k=v") {
		t.Errorf("handler b missed record: %q", b.buf.String())
	}
}

func TestMultiHandler_NilSkipped(t *testing.T) {
	a := &captureHandler{level: slog.LevelDebug, buf: &bytes.Buffer{}}
	h := NewMultiHandler(nil, a, nil)
	slog.New(h).Info("only one")
	if !strings.Contains(a.buf.String(), "only one") {
		t.Errorf("expected record in non-nil handler: %q", a.buf.String())
	}
}

func TestMultiHandler_EnabledIsAny(t *testing.T) {
	low := &captureHandler{level: slog.LevelDebug, buf: &bytes.Buffer{}}
	high := &captureHandler{level: slog.LevelError, buf: &bytes.Buffer{}}
	h := NewMultiHandler(low, high)

	// At INFO, only the debug handler is enabled but the multi-handler
	// should still report enabled (so the record reaches the eligible
	// inner handler).
	if !h.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("multi-handler should report enabled if any inner is")
	}

	slog.New(h).Info("info message")

	if !strings.Contains(low.buf.String(), "info message") {
		t.Errorf("low-threshold handler missed record: %q", low.buf.String())
	}
	if high.buf.Len() != 0 {
		t.Errorf("high-threshold handler unexpectedly received record: %q", high.buf.String())
	}
}

func TestMultiHandler_WithAttrsAppliesToAll(t *testing.T) {
	a := &captureHandler{level: slog.LevelDebug, buf: &bytes.Buffer{}}
	b := &captureHandler{level: slog.LevelDebug, buf: &bytes.Buffer{}}
	logger := slog.New(NewMultiHandler(a, b)).With("component", "ratelimit")
	logger.Info("hit")

	for i, h := range []*captureHandler{a, b} {
		if !strings.Contains(h.buf.String(), "component=ratelimit") {
			t.Errorf("handler %d missing With-attached attr: %q", i, h.buf.String())
		}
	}
}

func TestMultiHandler_OneFailureDoesNotSilenceOthers(t *testing.T) {
	failing := &captureHandler{level: slog.LevelDebug, buf: &bytes.Buffer{}, err: errors.New("disk full")}
	ok := &captureHandler{level: slog.LevelDebug, buf: &bytes.Buffer{}}
	h := NewMultiHandler(failing, ok)
	slog.New(h).Info("through")
	// ok handler should still have received the record even though the
	// failing one returned an error.
	if !strings.Contains(ok.buf.String(), "through") {
		t.Errorf("healthy handler did not receive record despite sibling error: %q", ok.buf.String())
	}
}

func TestMultiHandler_EmptyIsHarmless(t *testing.T) {
	h := NewMultiHandler()
	if h.Enabled(context.Background(), slog.LevelError) {
		t.Error("empty multi-handler should not report enabled at any level")
	}
	// Handle should not panic.
	if err := h.Handle(context.Background(), slog.Record{}); err != nil {
		t.Errorf("empty multi-handler Handle returned error: %v", err)
	}
}
