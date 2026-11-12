package integration

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// A client may send one message as a text frame plus continuation frames.
// golang.org/x/net/websocket can't send that, so these tests speak the
// websocket protocol on a raw connection.

type rawWS struct {
	t    *testing.T
	conn net.Conn
	r    *bufio.Reader
}

func dialRawWS(t *testing.T, relayURL string) *rawWS {
	t.Helper()
	u, err := url.Parse(relayURL)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	if err != nil {
		t.Fatalf("dial %s: %v", u.Host, err)
	}
	t.Cleanup(func() { conn.Close() })

	var key [16]byte
	rand.Read(key[:])
	fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"+
		"Sec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\nOrigin: http://localhost/\r\n\r\n",
		u.Host, base64.StdEncoding.EncodeToString(key[:]))
	r := bufio.NewReader(conn)
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	resp, err := http.ReadResponse(r, nil)
	if err != nil {
		t.Fatalf("websocket handshake: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("websocket handshake: status %d", resp.StatusCode)
	}
	return &rawWS{t: t, conn: conn, r: r}
}

// send writes msg as one message in frames of size bytes: a text frame,
// then continuation frames, FIN on the last. size 0 sends one frame. It
// reports whether every frame was written.
func (c *rawWS) send(msg string, size int) bool {
	if size <= 0 {
		size = len(msg)
	}
	for i := 0; i < len(msg) || i == 0; i += size {
		end := min(i+size, len(msg))
		op := byte(0x0) // continuation
		if i == 0 {
			op = 0x1 // text
		}
		if end == len(msg) {
			op |= 0x80 // FIN
		}
		if !c.writeFrame(op, []byte(msg[i:end])) {
			return false
		}
	}
	return true
}

// writeFrame writes one masked frame, as a client must.
func (c *rawWS) writeFrame(op byte, payload []byte) bool {
	h := []byte{op}
	switch n := len(payload); {
	case n < 126:
		h = append(h, 0x80|byte(n))
	case n <= 0xffff:
		h = append(h, 0x80|126, byte(n>>8), byte(n))
	default:
		h = append(h, 0x80|127)
		h = binary.BigEndian.AppendUint64(h, uint64(n))
	}
	var mask [4]byte
	rand.Read(mask[:])
	h = append(h, mask[:]...)
	masked := make([]byte, len(payload))
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}
	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := c.conn.Write(append(h, masked...))
	return err == nil
}

// read returns the next text message from the relay, or an error when the
// connection closes or nothing arrives before the deadline.
func (c *rawWS) read(deadline time.Time) ([]interface{}, error) {
	c.conn.SetReadDeadline(deadline)
	for {
		var h [2]byte
		if _, err := io.ReadFull(c.r, h[:]); err != nil {
			return nil, err
		}
		n := uint64(h[1] & 0x7f)
		switch n {
		case 126:
			var l [2]byte
			if _, err := io.ReadFull(c.r, l[:]); err != nil {
				return nil, err
			}
			n = uint64(binary.BigEndian.Uint16(l[:]))
		case 127:
			var l [8]byte
			if _, err := io.ReadFull(c.r, l[:]); err != nil {
				return nil, err
			}
			n = binary.BigEndian.Uint64(l[:])
		}
		p := make([]byte, n)
		if _, err := io.ReadFull(c.r, p); err != nil {
			return nil, err
		}
		switch h[0] & 0x0f {
		case 0x8:
			return nil, io.EOF // close frame
		case 0x1:
			var msg []interface{}
			if err := json.Unmarshal(p, &msg); err != nil {
				c.t.Fatalf("relay sent non-JSON text %q", p)
			}
			return msg, nil
		}
	}
}

// await reads until a message of the given type for id arrives (the
// relay's AUTH challenge and anything else are skipped).
func (c *rawWS) await(typ, id string, timeout time.Duration) []interface{} {
	c.t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		msg, err := c.read(deadline)
		if err != nil {
			c.t.Fatalf("waiting for %s %s: %v", typ, id, err)
		}
		if len(msg) >= 2 && msg[0] == typ && msg[1] == id {
			return msg
		}
	}
}

// An event sent as a text frame plus continuation frames is joined and
// stored. Frame edges land inside escapes, brackets and multi-byte
// characters.
func TestFragmented_EventAcrossFrames(t *testing.T) {
	c := dialRawWS(t, tests.DefaultRelayURL)
	kp := tests.NewTestKeypair()
	content := strings.Repeat(`a "quoted" back\slash [x] {y} é 日本 🙂`+"\n", 60)

	for _, size := range []int{100, 7} {
		evt := kp.SignEvent(1, fmt.Sprintf("%s%d", content, size), nil)
		b, _ := json.Marshal([]interface{}{"EVENT", evt})
		if !c.send(string(b), size) {
			t.Fatalf("frames of %d bytes: write failed", size)
		}
		ok := c.await("OK", evt.ID, 5*time.Second)
		if len(ok) < 3 || ok[2] != true {
			t.Fatalf("frames of %d bytes: got %v, want an accepted OK", size, ok)
		}
	}
}

// A frame that is not a nostr message must not hold back the messages
// after it. The relay used to keep every frame in one buffer until the
// buffer parsed as JSON, so after one bad frame nothing else ever did.
func TestFragmented_JunkFrameDoesNotBlockLaterMessages(t *testing.T) {
	c := dialRawWS(t, tests.DefaultRelayURL)
	c.send("not a nostr message", 0)

	subID := tests.RandomSubID()
	req, _ := json.Marshal([]interface{}{"REQ", subID, map[string]interface{}{"kinds": []int{1}, "limit": 1}})
	c.send(string(req), 0)
	c.await("EOSE", subID, 5*time.Second)
}

// A split message that grows past max_event_size (1 KiB on the ratelimit
// relay) closes the connection rather than being held.
func TestFragmented_OversizeMessageClosesConnection(t *testing.T) {
	c := dialRawWS(t, tests.RateLimitRelayURL)
	msg := `["EVENT",{"content":"` + strings.Repeat("a", 4096) + `"}]`
	c.send(msg, 256) // writes may fail once the relay hangs up

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := c.read(deadline); err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				t.Fatal("relay kept the connection open after an oversize message")
			}
			return // closed
		}
	}
}
