package core

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	nostr "github.com/0ceanslim/grain/server/types"
)

// fragmentingRelay is a relay on a hand-made websocket, so it can send one
// message as a text frame plus continuation frames, which
// golang.org/x/net/websocket never does on its own. Every REQ is answered
// with one signed event, sent in frames of frameSize bytes, then an EOSE in
// one frame.
type fragmentingRelay struct {
	url       string
	signer    *EventSigner
	frameSize int
	content   string
}

func newFragmentingRelay(t *testing.T, frameSize int, content string) *fragmentingRelay {
	t.Helper()
	signer, err := NewEventSignerFromRandom()
	if err != nil {
		t.Fatal(err)
	}
	r := &fragmentingRelay{signer: signer, frameSize: frameSize, content: content}
	srv := httptest.NewServer(http.HandlerFunc(r.serve))
	t.Cleanup(srv.Close)
	r.url = "ws://" + strings.TrimPrefix(srv.URL, "http://")
	return r
}

func (r *fragmentingRelay) serve(w http.ResponseWriter, req *http.Request) {
	accept := sha1.Sum([]byte(req.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	conn, buf, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return
	}
	defer conn.Close()
	fmt.Fprintf(buf, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n",
		base64.StdEncoding.EncodeToString(accept[:]))
	buf.Flush()

	for {
		msg, ok := readClientFrame(buf)
		if !ok {
			return
		}
		var frame []json.RawMessage
		if json.Unmarshal([]byte(msg), &frame) != nil || len(frame) < 2 {
			continue
		}
		var typ, subID string
		_ = json.Unmarshal(frame[0], &typ)
		_ = json.Unmarshal(frame[1], &subID)
		if typ != "REQ" {
			continue
		}
		evt := nostr.Event{Kind: 1, CreatedAt: time.Now().Unix(), Tags: [][]string{}, Content: r.content}
		if err := r.signer.SignEvent(&evt); err != nil {
			return // the test sees no event
		}
		writeServerMessage(buf, r.frameSize, "EVENT", subID, evt)
		writeServerMessage(buf, 0, "EOSE", subID)
	}
}

// readClientFrame reads one masked frame from the client (the client sends
// each message as a single frame).
func readClientFrame(buf *bufio.ReadWriter) (string, bool) {
	var h [2]byte
	if _, err := io.ReadFull(buf, h[:]); err != nil || h[0]&0x0f == 0x8 {
		return "", false
	}
	n := uint64(h[1] & 0x7f)
	switch n {
	case 126:
		var l [2]byte
		if _, err := io.ReadFull(buf, l[:]); err != nil {
			return "", false
		}
		n = uint64(binary.BigEndian.Uint16(l[:]))
	case 127:
		var l [8]byte
		if _, err := io.ReadFull(buf, l[:]); err != nil {
			return "", false
		}
		n = binary.BigEndian.Uint64(l[:])
	}
	var mask [4]byte
	if _, err := io.ReadFull(buf, mask[:]); err != nil {
		return "", false
	}
	p := make([]byte, n)
	if _, err := io.ReadFull(buf, p); err != nil {
		return "", false
	}
	for i := range p {
		p[i] ^= mask[i%4]
	}
	return string(p), true
}

// writeServerMessage sends v as one message split into frames of frameSize
// bytes (one frame if frameSize is 0): a text frame, then continuation
// frames, FIN set on the last.
func writeServerMessage(buf *bufio.ReadWriter, frameSize int, v ...interface{}) {
	b, _ := json.Marshal(v)
	if frameSize <= 0 {
		frameSize = len(b)
	}
	for i := 0; i < len(b); i += frameSize {
		end := min(i+frameSize, len(b))
		op := byte(0x0) // continuation
		if i == 0 {
			op = 0x1 // text
		}
		if end == len(b) {
			op |= 0x80 // FIN
		}
		part := b[i:end]
		switch {
		case len(part) < 126:
			buf.Write([]byte{op, byte(len(part))})
		case len(part) <= 0xffff:
			buf.Write([]byte{op, 126, byte(len(part) >> 8), byte(len(part))})
		default:
			h := []byte{op, 127, 0, 0, 0, 0, 0, 0, 0, 0}
			binary.BigEndian.PutUint64(h[2:], uint64(len(part)))
			buf.Write(h)
		}
		buf.Write(part)
	}
	buf.Flush()
}

// Regression: websocket.Message.Receive returns one frame per call, so an
// event a relay sent as a text frame plus a continuation frame reached
// processMessage as two halves, neither of which parsed, and was dropped.
// Some relays split any message over 1 KiB this way.
func TestRelayMessageSplitAcrossFrames(t *testing.T) {
	// Escapes, brackets, quotes and multi-byte characters, so frame edges
	// land inside all of them.
	content := strings.Repeat("a \"quoted\" back\\slash [x] {y} é 日本 🙂\n", 40)
	for _, frameSize := range []int{0, 1024, 100, 7} {
		name := fmt.Sprintf("frames of %d bytes", frameSize)
		if frameSize == 0 {
			name = "one frame"
		}
		t.Run(name, func(t *testing.T) {
			relay := newFragmentingRelay(t, frameSize, content)
			c := NewClient(DefaultConfig())
			defer c.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			filter := nostr.Filter{Kinds: []int{1}, Authors: []string{relay.signer.publicKey}}
			evs := c.QueryEvents(ctx, filter, []string{relay.url}, WithTimeout(5*time.Second))
			if len(evs) != 1 {
				t.Fatalf("got %d events, want 1 (the event the relay split across frames)", len(evs))
			}
			if evs[0].Content != content {
				t.Fatalf("event content is %d bytes, want %d", len(evs[0].Content), len(content))
			}
		})
	}
}

func TestFrameJoiner(t *testing.T) {
	type step struct {
		frame string
		want  string
		ok    bool // a whole message is returned
	}
	cases := []struct {
		name  string
		max   int
		steps []step
	}{
		{"whole message", 0, []step{
			{`["EOSE","s"]`, `["EOSE","s"]`, true},
		}},
		{"split in two", 0, []step{
			{`["EVENT","s",{"content":"he`, "", false},
			{`llo"}]`, `["EVENT","s",{"content":"hello"}]`, true},
		}},
		{"middle frame is valid JSON on its own", 0, []step{
			{`["EVENT","s",{"tags":[`, "", false},
			{`["p","abc"]`, "", false},
			{`]}]`, `["EVENT","s",{"tags":[["p","abc"]]}]`, true},
		}},
		{"brackets and an escaped quote inside a string", 0, []step{
			{`["EVENT","s",{"content":"] } \`, "", false},
			{`" ["}]`, `["EVENT","s",{"content":"] } \" ["}]`, true},
		}},
		{"text that is not JSON passes through and holds nothing", 0, []step{
			{`OK`, `OK`, true},
			{``, ``, true},
			{`["EOSE","s"]`, `["EOSE","s"]`, true},
		}},
		{"an object or bare string is not held", 0, []step{
			{`{bad`, `{bad`, true},
			{`"unterminated`, `"unterminated`, true},
			{`["EOSE","s"]`, `["EOSE","s"]`, true},
		}},
		{"a message that never closes is dropped at max", 30, []step{
			{`["EVENT","s",{"content":"`, "", false},
			{`aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`, `aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa`, true},
			{`["EOSE","s"]`, `["EOSE","s"]`, true},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j := frameJoiner{max: tc.max, gap: frameGap}
			if j.max == 0 {
				j.max = maxHeldMessage
			}
			at := time.Now()
			for i, st := range tc.steps {
				got, ok := j.add(st.frame, at)
				at = at.Add(time.Millisecond)
				if ok != st.ok || got != st.want {
					t.Fatalf("frame %d: got (%q, %v), want (%q, %v)", i, got, ok, st.want, st.ok)
				}
			}
		})
	}
}

// Every way of cutting a message into two or three frames joins back to
// the message, and the joiner is ready for the next one.
func TestFrameJoinerEverySplit(t *testing.T) {
	msg := `["EVENT","s",{"content":"a \"q\" \\ [x] {y} é 日本 🙂","created_at":1712345678,"tags":[["p","abc"],["t","x"]]}]`
	next := `["EOSE","s"]`
	check := func(t *testing.T, frames ...string) {
		t.Helper()
		j := frameJoiner{max: maxHeldMessage, gap: frameGap}
		now := time.Now()
		for i, f := range frames {
			got, ok := j.add(f, now)
			last := i == len(frames)-1
			if ok != last || (last && got != msg) {
				t.Fatalf("frames %q: frame %d gave (%q, %v)", frames, i, got, ok)
			}
		}
		if got, ok := j.add(next, now); !ok || got != next {
			t.Fatalf("frames %q: next message gave (%q, %v)", frames, got, ok)
		}
	}
	for a := 1; a < len(msg); a++ {
		check(t, msg[:a], msg[a:])
		for b := a + 1; b < len(msg); b++ {
			check(t, msg[:a], msg[a:b], msg[b:])
		}
	}
}

// A held message whose next frame doesn't come within the gap was never
// finished (a truncated or broken message): it is dropped, and the next
// message comes through.
func TestFrameJoinerDropsStaleHeldText(t *testing.T) {
	j := frameJoiner{max: maxHeldMessage, gap: frameGap}
	start := time.Now()
	if _, ok := j.add(`["EVENT","s",{"content":"unterminated`, start); ok {
		t.Fatal("an unfinished message was returned")
	}
	got, ok := j.add(`["EOSE","s"]`, start.Add(frameGap+time.Second))
	if !ok || got != `["EOSE","s"]` {
		t.Fatalf("after the gap got (%q, %v), want the EOSE", got, ok)
	}
}
