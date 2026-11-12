package wsframe

import (
	"strings"
	"testing"
	"time"
)

func newJoiner(max int) *Joiner {
	if max == 0 {
		max = DefaultMax
	}
	return &Joiner{Max: max, Gap: DefaultGap}
}

func TestJoiner(t *testing.T) {
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
		{"a first frame already past max is dropped", 10, []step{
			{`["EVENT","s",{"content":"`, "", false},
			{`["EOSE","s"]`, `["EOSE","s"]`, true},
		}},
		{"a whole message past max still passes", 10, []step{
			{`["EVENT","s",{"content":"x"}]`, `["EVENT","s",{"content":"x"}]`, true},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j := newJoiner(tc.max)
			at := time.Now()
			for i, st := range tc.steps {
				got, ok := j.Add(st.frame, at)
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
func TestJoinerEverySplit(t *testing.T) {
	msg := `["EVENT","s",{"content":"a \"q\" \\ [x] {y} é 日本 🙂","created_at":1712345678,"tags":[["p","abc"],["t","x"]]}]`
	next := `["EOSE","s"]`
	check := func(t *testing.T, frames ...string) {
		t.Helper()
		j := newJoiner(0)
		now := time.Now()
		for i, f := range frames {
			got, ok := j.Add(f, now)
			last := i == len(frames)-1
			if ok != last || (last && got != msg) {
				t.Fatalf("frames %q: frame %d gave (%q, %v)", frames, i, got, ok)
			}
		}
		if got, ok := j.Add(next, now); !ok || got != next {
			t.Fatalf("frames %q: next message gave (%q, %v)", frames, got, ok)
		}
	}
	for a := 1; a < len(msg); a++ {
		check(t, msg[:a], msg[a:])
		for b := a + 1; b < len(msg); b++ {
			check(t, msg[:a], msg[a:b], msg[b:])
		}
	}
	// One byte per frame.
	frames := make([]string, len(msg))
	for i := 0; i < len(msg); i++ {
		frames[i] = msg[i : i+1]
	}
	check(t, frames...)
}

// A held message whose next frame doesn't come within the gap was never
// finished (a truncated or broken message): it is dropped, and the next
// message comes through.
func TestJoinerDropsStaleHeldText(t *testing.T) {
	j := newJoiner(0)
	var drops []string
	j.OnDrop = func(held int, reason string) { drops = append(drops, reason) }
	start := time.Now()
	if _, ok := j.Add(`["EVENT","s",{"content":"unterminated`, start); ok {
		t.Fatal("an unfinished message was returned")
	}
	got, ok := j.Add(`["EOSE","s"]`, start.Add(DefaultGap+time.Second))
	if !ok || got != `["EOSE","s"]` {
		t.Fatalf("after the gap got (%q, %v), want the EOSE", got, ok)
	}
	if len(drops) != 1 || drops[0] != DropStale {
		t.Fatalf("OnDrop calls = %q, want one %q", drops, DropStale)
	}
}

// OnDrop hears about both ways a message gets too large: a held message
// that grows past Max, and a first frame that is already past it.
func TestJoinerReportsTooLarge(t *testing.T) {
	type drop struct {
		held   int
		reason string
	}
	var drops []drop
	j := newJoiner(30)
	j.OnDrop = func(held int, reason string) { drops = append(drops, drop{held, reason}) }
	now := time.Now()

	first := `["EVENT","s",{"content":"` // held
	j.Add(first, now)
	j.Add(strings.Repeat("a", 10), now) // grows it past 30
	big := first + strings.Repeat("b", 10)
	j.Add(big, now)

	want := []drop{{len(first), DropTooLarge}, {len(big), DropTooLarge}}
	if len(drops) != len(want) || drops[0] != want[0] || drops[1] != want[1] {
		t.Fatalf("OnDrop calls = %+v, want %+v", drops, want)
	}
}

// Joining a large message sent in small frames is linear: each frame scans
// only its own bytes. A 1 MiB message in 1 KiB frames should take a few ms.
func BenchmarkJoiner1MiBIn1KiBFrames(b *testing.B) {
	msg := `["EVENT","s",{"content":"` + strings.Repeat("x", DefaultMax-64) + `"}]`
	const size = 1024
	b.SetBytes(int64(len(msg)))
	for i := 0; i < b.N; i++ {
		j := newJoiner(0)
		now := time.Now()
		var ok bool
		for k := 0; k < len(msg); k += size {
			_, ok = j.Add(msg[k:min(k+size, len(msg))], now)
		}
		if !ok {
			b.Fatal("message not joined")
		}
	}
}
