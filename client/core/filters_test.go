package core

import (
	"encoding/json"
	"testing"
	"time"
)

// Regression: the REQ a FilterBuilder filter produces must be NIP-01 on the
// wire. It used to send {"#":{"#d":[...]}} and an RFC3339 "since", so loading
// addressable events by d tag (or anything by p tag / time window) silently
// returned the wrong set from every remote relay.
func TestFilterBuilder_REQWireFormat(t *testing.T) {
	f := NewFilterBuilder().
		Kinds(30078).
		Authors("abc").
		Tag("d", "save-1").
		Tag("#p", "def").
		Since(time.Unix(1700000000, 0)).
		Until(time.Unix(1700003600, 0)).
		Limit(3).
		Build()

	b, err := json.Marshal([]interface{}{"REQ", "sub1", f})
	if err != nil {
		t.Fatal(err)
	}
	const want = `["REQ","sub1",{"#d":["save-1"],"#p":["def"],"authors":["abc"],"kinds":[30078],"limit":3,"since":1700000000,"until":1700003600}]`
	if string(b) != want {
		t.Fatalf("REQ wire format\n got: %s\nwant: %s", b, want)
	}
}

func TestFilterBuilder_TagStoredBare(t *testing.T) {
	f := NewFilterBuilder().Tag("d", "a").Tag("#d", "b").Build()
	if got := f.Tags["d"]; len(got) != 2 {
		t.Fatalf(`Tags["d"] = %v, want both values under the bare key`, got)
	}
	if _, ok := f.Tags["#d"]; ok {
		t.Fatal(`Tags has a "#d" key; keys must be bare tag names`)
	}
}
