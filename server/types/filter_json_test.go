package relay

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// decodeWire marshals v and decodes the result generically, so assertions
// check the JSON a relay actually receives rather than Go struct layout.
func decodeWire(t *testing.T, v interface{}) map[string]interface{} {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", b, err)
	}
	return m
}

// Regression: Filter used to marshal tag filters as a nested {"#": {...}}
// object and since/until as RFC3339 strings, so every #d / #p / since / until
// constraint sent to a remote relay was ignored or rejected.
func TestFilterMarshalJSON_NIP01WireFormat(t *testing.T) {
	since := time.Unix(1700000000, 0).In(time.FixedZone("EST", -5*3600))
	until := time.Unix(1700003600, 0)
	limit := 5
	f := Filter{
		IDs:     []string{"aa"},
		Authors: []string{"bb"},
		Kinds:   []int{30078},
		Tags:    map[string][]string{"d": {"save-1"}, "#p": {"cc"}},
		Since:   &since,
		Until:   &until,
		Limit:   &limit,
		Search:  "hello",
	}

	got := decodeWire(t, f)
	want := map[string]interface{}{
		"ids":     []interface{}{"aa"},
		"authors": []interface{}{"bb"},
		"kinds":   []interface{}{float64(30078)},
		"#d":      []interface{}{"save-1"},
		"#p":      []interface{}{"cc"},
		"since":   float64(1700000000),
		"until":   float64(1700003600),
		"limit":   float64(5),
		"search":  "hello",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wire format mismatch\n got: %v\nwant: %v", got, want)
	}
}

// A pointer, a value, and a filter inside a REQ array must all encode the
// same way (MarshalJSON has a value receiver).
func TestFilterMarshalJSON_InsideREQArray(t *testing.T) {
	since := time.Unix(42, 0)
	f := Filter{Kinds: []int{1}, Tags: map[string][]string{"e": {"x"}}, Since: &since}
	b, err := json.Marshal([]interface{}{"REQ", "s", f, &f})
	if err != nil {
		t.Fatal(err)
	}
	const want = `["REQ","s",{"#e":["x"],"kinds":[1],"since":42},{"#e":["x"],"kinds":[1],"since":42}]`
	if string(b) != want {
		t.Fatalf("got  %s\nwant %s", b, want)
	}
}

func TestFilterMarshalJSON_OmitsUnsetFields(t *testing.T) {
	b, err := json.Marshal(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "{}" {
		t.Fatalf("empty filter = %s, want {}", b)
	}
}

// An explicitly empty list means "match nothing" in NIP-01. Dropping it on
// encode would widen the filter to "match everything".
func TestFilterMarshalJSON_KeepsEmptyLists(t *testing.T) {
	got := decodeWire(t, Filter{
		Kinds: []int{},
		Tags:  map[string][]string{"d": {}, "p": nil},
	})
	if v, ok := got["kinds"]; !ok || len(v.([]interface{})) != 0 {
		t.Errorf("kinds = %v (present=%v), want []", v, ok)
	}
	if v, ok := got["#d"]; !ok || len(v.([]interface{})) != 0 {
		t.Errorf("#d = %v (present=%v), want []", v, ok)
	}
	if _, ok := got["#p"]; ok {
		t.Errorf("nil #p list should be omitted")
	}
}

func TestFilterJSON_RoundTrip(t *testing.T) {
	since := time.Unix(1700000000, 0)
	limit := 10
	in := Filter{
		Authors: []string{"ab"},
		Kinds:   []int{1, 30078},
		Tags:    map[string][]string{"d": {"a", "b"}},
		Since:   &since,
		Limit:   &limit,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Filter
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", b, err)
	}
	if !reflect.DeepEqual(out.Authors, in.Authors) || !reflect.DeepEqual(out.Kinds, in.Kinds) ||
		!reflect.DeepEqual(out.Tags, in.Tags) || *out.Limit != limit ||
		out.Since == nil || !out.Since.Equal(since) {
		t.Fatalf("round trip mismatch: %s -> %+v", b, out)
	}
}

func TestFilterUnmarshalJSON_Strict(t *testing.T) {
	var f Filter
	if err := json.Unmarshal([]byte(`{"kinds":["1"]}`), &f); err == nil {
		t.Fatal("expected error for string kinds")
	}
	if err := json.Unmarshal([]byte(`{"since":"2023-11-14T17:13:20Z"}`), &f); err == nil {
		t.Fatal("expected error for RFC3339 since")
	}
}

// Tags keys are bare names; a "#d" key (the old FilterBuilder spelling) must
// match the same events as "d".
func TestFilterMatchesEvent_TagKeyHashTolerated(t *testing.T) {
	evt := Event{Kind: 30078, Tags: [][]string{{"d", "save-1"}}}
	for _, key := range []string{"d", "#d"} {
		f := Filter{Tags: map[string][]string{key: {"save-1"}}}
		if !f.MatchesEvent(evt) {
			t.Errorf("key %q: expected match", key)
		}
		f = Filter{Tags: map[string][]string{key: {"other"}}}
		if f.MatchesEvent(evt) {
			t.Errorf("key %q: expected no match", key)
		}
	}
}

func TestToSubscriptionFilter_NoDoubleHash(t *testing.T) {
	f := Filter{Tags: map[string][]string{"#d": {"x"}, "p": {"y"}}}
	m := f.ToSubscriptionFilter()
	if _, ok := m["##d"]; ok {
		t.Fatal("produced ##d key")
	}
	if _, ok := m["#d"]; !ok {
		t.Fatal("missing #d key")
	}
	if _, ok := m["#p"]; !ok {
		t.Fatal("missing #p key")
	}
}
