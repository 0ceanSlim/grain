package relay

import "strconv"

// NIP-01 JSON encoding for events.
//
// encoding/json escapes &, <, >, U+2028, U+2029 and most control characters
// as \uXXXX. NIP-01 defines the id preimage with exactly seven escapes and
// every other byte verbatim, and nostrdb's parser rejects \uXXXX outright.
// Anything that is hashed for an id or handed to nostrdb must go through
// these encoders instead of json.Marshal: an event whose content held an `&`
// used to reach nostrdb as \u0026, fail to parse in the async ingester, and
// vanish after grain had already acked it.

// Commitment returns the NIP-01 id preimage
// [0,<pubkey>,<created_at>,<kind>,<tags>,<content>]; the event id is its
// SHA-256.
func (e Event) Commitment() []byte {
	b := make([]byte, 0, 96+len(e.Content)+tagsSize(e.Tags))
	b = append(b, "[0,"...)
	b = appendNIP01String(b, e.PubKey)
	b = append(b, ',')
	b = strconv.AppendInt(b, e.CreatedAt, 10)
	b = append(b, ',')
	b = strconv.AppendInt(b, int64(e.Kind), 10)
	b = append(b, ',')
	b = appendNIP01Tags(b, e.Tags)
	b = append(b, ',')
	b = appendNIP01String(b, e.Content)
	return append(b, ']')
}

// CanonicalJSON returns the event as a JSON object whose strings use the
// NIP-01 escapes, so a parser that re-derives the id from it (nostrdb's
// ingester does) hashes the same bytes as Commitment. Nil tags encode as [].
func (e Event) CanonicalJSON() []byte {
	b := make([]byte, 0, 320+len(e.Content)+tagsSize(e.Tags))
	b = append(b, `{"id":`...)
	b = appendNIP01String(b, e.ID)
	b = append(b, `,"pubkey":`...)
	b = appendNIP01String(b, e.PubKey)
	b = append(b, `,"created_at":`...)
	b = strconv.AppendInt(b, e.CreatedAt, 10)
	b = append(b, `,"kind":`...)
	b = strconv.AppendInt(b, int64(e.Kind), 10)
	b = append(b, `,"tags":`...)
	b = appendNIP01Tags(b, e.Tags)
	b = append(b, `,"content":`...)
	b = appendNIP01String(b, e.Content)
	b = append(b, `,"sig":`...)
	b = appendNIP01String(b, e.Sig)
	return append(b, '}')
}

func appendNIP01Tags(b []byte, tags [][]string) []byte {
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
			b = appendNIP01String(b, s)
		}
		b = append(b, ']')
	}
	return append(b, ']')
}

// appendNIP01String appends s as a quoted JSON string, escaping only the
// seven characters NIP-01 lists. All of them are ASCII, so a byte scan never
// splits a multi-byte UTF-8 sequence.
func appendNIP01String(b []byte, s string) []byte {
	b = append(b, '"')
	start := 0
	for i := 0; i < len(s); i++ {
		var esc string
		switch s[i] {
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
