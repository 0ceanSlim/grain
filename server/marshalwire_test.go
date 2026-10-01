package server

import (
	"strings"
	"testing"
)

// nostrdb-based clients (Damus, Notedeck) drop any relay message holding a
// \u escape, so the frames grain sends must carry &, < and > raw.
func TestSendMessageBlocking_NoHTMLEscaping(t *testing.T) {
	c, cancel := stubClient(t, 1)
	defer cancel()

	content := "a & b <c> https://x.com/?a=1&b=2"
	msg := []interface{}{"EVENT", "sub", map[string]interface{}{"content": content}}
	if err := c.SendMessageBlocking(msg); err != nil {
		t.Fatalf("send: %v", err)
	}
	got := string(<-c.outgoing)
	if !strings.Contains(got, content) || strings.Contains(got, `\u00`) {
		t.Fatalf("frame escapes HTML characters: %s", got)
	}
	if strings.HasSuffix(got, "\n") {
		t.Fatalf("frame has a trailing newline: %q", got)
	}
}
