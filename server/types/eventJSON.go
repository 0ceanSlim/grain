package relay

import (
	"encoding/json"
	"strconv"
)

// NIP-01 JSON encoding for events.
//
// encoding/json escapes &, <, >, U+2028, U+2029 and control characters as
// \uXXXX, which matches neither the id preimage clients sign nor what
// nostrdb can parse. Anything hashed for an id or handed to nostrdb goes
// through these encoders instead: an event whose content held an `&` used to
// reach nostrdb as \u0026, fail to parse in the async ingester, and vanish
// after grain had already acked it.

// Commitment returns the NIP-01 id preimage
// [0,<pubkey>,<created_at>,<kind>,<tags>,<content>]; the event id is its
// SHA-256.
func (e Event) Commitment() []byte {
	b := make([]byte, 0, 96+len(e.Content)+tagsSize(e.Tags))
	b = append(b, "[0,"...)
	b = appendJSONString(b, e.PubKey, true)
	b = append(b, ',')
	b = strconv.AppendInt(b, e.CreatedAt, 10)
	b = append(b, ',')
	b = strconv.AppendInt(b, int64(e.Kind), 10)
	b = append(b, ',')
	b = appendJSONTags(b, e.Tags, true)
	b = append(b, ',')
	b = appendJSONString(b, e.Content, true)
	return append(b, ']')
}

// CanonicalJSON returns the event as a JSON object for nostrdb's ingester.
// It escapes like Commitment except that control characters stay raw:
// nostrdb's parser can't read \u escapes but takes raw control characters,
// and its own id derivation escapes them the way Commitment does, so the id
// it computes matches. Nil tags encode as [].
func (e Event) CanonicalJSON() []byte {
	b := make([]byte, 0, 320+len(e.Content)+tagsSize(e.Tags))
	b = append(b, `{"id":`...)
	b = appendJSONString(b, e.ID, false)
	b = append(b, `,"pubkey":`...)
	b = appendJSONString(b, e.PubKey, false)
	b = append(b, `,"created_at":`...)
	b = strconv.AppendInt(b, e.CreatedAt, 10)
	b = append(b, `,"kind":`...)
	b = strconv.AppendInt(b, int64(e.Kind), 10)
	b = append(b, `,"tags":`...)
	b = appendJSONTags(b, e.Tags, false)
	b = append(b, `,"content":`...)
	b = appendJSONString(b, e.Content, false)
	b = append(b, `,"sig":`...)
	b = appendJSONString(b, e.Sig, false)
	return append(b, '}')
}

func appendJSONTags(b []byte, tags [][]string, escapeControl bool) []byte {
	b = append(b, '[')
	for i, tag := range tags {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, '[')
		for j, s := range tag {
			if j > 0 {
				b = append(b, ',')
			}
			b = appendJSONString(b, s, escapeControl)
		}
		b = append(b, ']')
	}
	return append(b, ']')
}

// appendJSONString appends s as a quoted JSON string the way id preimages
// are written in practice: ", \ and the five control characters with short
// escapes (\b \t \n \f \r) are escaped, and with escapeControl the other 27
// control characters become lowercase \u00XX — what JSON.stringify (so
// nostr-tools), go-nostr and rust-nostr all emit. NIP-01's text says those 27
// go in verbatim, but no major client signs them that way. Every other byte,
// including DEL and U+2028/U+2029, is verbatim. All escaped characters are
// ASCII, so a byte scan never splits a multi-byte UTF-8 sequence.
func appendJSONString(b []byte, s string, escapeControl bool) []byte {
	const hexDigits = "0123456789abcdef"
	b = append(b, '"')
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		var esc string
		switch c {
		case '"':
			esc = `\"`
		case '\\':
			esc = `\\`
		case '\n':
			esc = `\n`
		case '\r':
			esc = `\r`
		case '\t':
			esc = `\t`
		case '\b':
			esc = `\b`
		case '\f':
			esc = `\f`
		default:
			if c >= 0x20 || !escapeControl {
				continue
			}
			b = append(b, s[start:i]...)
			b = append(b, '\\', 'u', '0', '0', hexDigits[c>>4], hexDigits[c&0xf])
			start = i + 1
			continue
		}
		b = append(b, s[start:i]...)
		b = append(b, esc...)
		start = i + 1
	}
	b = append(b, s[start:]...)
	return append(b, '"')
}

func tagsSize(tags [][]string) int {
	n := 0
	for _, tag := range tags {
		for _, s := range tag {
			n += len(s) + 3
		}
		n += 2
	}
	return n
}

// ForLog returns the event as JSON for a log line, cut to max bytes.
// encoding/json escapes control characters and U+2028/U+2029, so the exact
// characters an id disagreement turns on stay visible in the log.
func (e Event) ForLog(max int) string {
	b, err := json.Marshal(e)
	if err != nil {
		return "unencodable event: " + err.Error()
	}
	if len(b) > max {
		return string(b[:max]) + "...(" + strconv.Itoa(len(b)-max) + " more bytes)"
	}
	return string(b)
}
