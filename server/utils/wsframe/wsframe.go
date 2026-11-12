// Package wsframe puts back together a nostr message that arrives split
// across websocket frames.
//
// golang.org/x/net/websocket's Message.Receive returns one frame per call,
// and a continuation frame comes back as if it were a message of its own. So
// a message sent as a text frame plus continuation frames (some relays do
// this for anything over 1 KiB) arrives in pieces that don't parse. grain
// reads this way on both sides: the relay reading from clients, and the
// client library reading from relays.
//
// x/net/websocket doesn't expose the FIN bit, so the end of a message is
// found from the JSON itself. A frame that opens a JSON array (every nostr
// message is one) and leaves it open is held, and the frames after it are
// added until the brackets close. Brackets inside strings (and escaped
// quotes) are skipped, and each byte is scanned once, so joining costs no
// more than reading. Other frames pass through as they are.
//
// A broken frame that leaves a bracket or a string open would hold back
// what follows it, so held text is dropped once it passes Max bytes, or
// when the next frame comes more than Gap after the last (the frames of one
// message arrive back to back).
package wsframe

import (
	"strings"
	"time"
)

const (
	// DefaultMax is the most bytes held for one message when the caller
	// has no limit of its own.
	DefaultMax = 1 << 20
	// DefaultGap is how long held text waits for its next frame.
	DefaultGap = 5 * time.Second
)

// Reasons passed to Joiner.OnDrop.
const (
	DropTooLarge = "too large" // the message passed Max
	DropStale    = "stale"     // the next frame came more than Gap later
)

// Joiner joins the frames of one connection. The zero value holds nothing
// (Max 0); set Max and Gap before use. Not safe for concurrent use: each
// connection's read loop owns its own.
type Joiner struct {
	Max int           // most bytes held for one message
	Gap time.Duration // longest wait between frames of one message

	// OnDrop, if set, is told when held text is given up on: how many
	// bytes, and DropTooLarge or DropStale.
	OnDrop func(held int, reason string)

	pending strings.Builder
	lastAt  time.Time // when the last held frame arrived
	depth   int       // open brackets in pending
	inStr   bool      // pending ends inside a JSON string
	esc     bool      // pending ends just after a backslash in a string
}

// Add takes the next frame, received at now, and returns a whole message
// once one is complete.
func (j *Joiner) Add(frame string, now time.Time) (string, bool) {
	if j.pending.Len() > 0 {
		switch {
		case j.pending.Len()+len(frame) > j.Max:
			j.drop(DropTooLarge)
		case now.Sub(j.lastAt) > j.Gap:
			j.drop(DropStale)
		default:
			j.pending.WriteString(frame)
			j.lastAt = now
			if !j.scan(frame) {
				return "", false
			}
			message := j.pending.String()
			j.reset()
			return message, true
		}
		// The held message never finished; frame starts afresh.
	}
	t := strings.TrimLeft(frame, " \t\r\n")
	if t == "" || t[0] != '[' || j.scan(frame) {
		j.reset()
		return frame, true
	}
	if len(frame) > j.Max {
		// The start of a message already too large to hold.
		j.reset()
		j.dropped(len(frame), DropTooLarge)
		return "", false
	}
	j.pending.WriteString(frame)
	j.lastAt = now
	return "", false
}

// scan advances the bracket count over s and reports whether every bracket
// opened so far has closed.
func (j *Joiner) scan(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case j.esc:
			j.esc = false
		case j.inStr:
			if c == '\\' {
				j.esc = true
			} else if c == '"' {
				j.inStr = false
			}
		case c == '"':
			j.inStr = true
		case c == '[' || c == '{':
			j.depth++
		case c == ']' || c == '}':
			j.depth--
		}
	}
	return j.depth <= 0
}

func (j *Joiner) drop(reason string) {
	held := j.pending.Len()
	j.reset()
	j.dropped(held, reason)
}

func (j *Joiner) dropped(held int, reason string) {
	if j.OnDrop != nil {
		j.OnDrop(held, reason)
	}
}

func (j *Joiner) reset() {
	j.pending.Reset()
	j.depth, j.inStr, j.esc = 0, false, false
}
