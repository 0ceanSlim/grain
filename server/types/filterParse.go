package relay

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// ParseFilter builds a Filter from one decoded NIP-01 filter object. It is
// strict on purpose: a malformed field is an error, never silently skipped,
// because skipping a constraint WIDENS the filter — a typo'd {"kinds":["1"]}
// used to lose its kinds and match every event in the database.
//
// matchable=false with a nil error means the filter is well-formed but can
// never match anything (an empty ids/authors/kinds/#tag list, same as
// nostr-tools and go-nostr); callers drop it instead of querying.
//
// ids and authors accept 1–64 hex chars. Anything shorter than 64 is matched
// as a prefix — nonstandard since NIP-01 moved to exact ids, but deliberately
// kept: tools like glyphbyte.dev look events up by their leading bytes. See
// HasPrefixMatch.
func ParseFilter(raw map[string]interface{}) (f Filter, matchable bool, err error) {
	matchable = true
	f.Tags = make(map[string][]string) // callers add tag constraints (e.g. COUNT's p-tag gate)

	for key, val := range raw {
		switch {
		case key == "ids" || key == "authors":
			list, err := parseHexPrefixes(key, val)
			if err != nil {
				return Filter{}, false, err
			}
			if len(list) == 0 {
				matchable = false
			}
			if key == "ids" {
				f.IDs = list
			} else {
				f.Authors = list
			}

		case key == "kinds":
			kinds, err := parseKinds(val)
			if err != nil {
				return Filter{}, false, err
			}
			if len(kinds) == 0 {
				matchable = false
			}
			f.Kinds = kinds

		case key == "since" || key == "until":
			ts, err := parseNonNegative(key, val)
			if err != nil {
				return Filter{}, false, err
			}
			t := time.Unix(ts, 0).UTC()
			if key == "since" {
				f.Since = &t
			} else {
				f.Until = &t
			}

		case key == "limit":
			n, err := parseNonNegative(key, val)
			if err != nil {
				return Filter{}, false, err
			}
			if n > math.MaxInt32 {
				n = math.MaxInt32 // capped to the implicit limit downstream anyway
			}
			limit := int(n)
			f.Limit = &limit

		case key == "search": // NIP-50
			s, ok := val.(string)
			if !ok {
				return Filter{}, false, fmt.Errorf("search must be a string")
			}
			f.Search = s

		case len(key) >= 2 && key[0] == '#':
			vals, err := parseStrings(key, val)
			if err != nil {
				return Filter{}, false, err
			}
			if len(vals) == 0 {
				matchable = false
			}
			f.Tags[key[1:]] = vals
		}
		// Unknown keys are ignored for forward compatibility.
	}

	return f, matchable, nil
}

// HasPrefixMatch reports whether any ids/authors entry is shorter than a full
// 64-char id, i.e. will be matched as a prefix rather than exactly.
func (f Filter) HasPrefixMatch() bool {
	for _, id := range f.IDs {
		if len(id) < 64 {
			return true
		}
	}
	for _, pk := range f.Authors {
		if len(pk) < 64 {
			return true
		}
	}
	return false
}

// parseHexPrefixes validates an ids/authors list: each entry 1–64 hex chars.
// Uppercase hex is accepted and lowered, since stored ids are lowercase.
func parseHexPrefixes(key string, val interface{}) ([]string, error) {
	vals, err := parseStrings(key, val)
	if err != nil {
		return nil, err
	}
	for i, v := range vals {
		if len(v) == 0 || len(v) > 64 {
			return nil, fmt.Errorf("%s[%d] must be 1-64 hex characters, got %d", key, i, len(v))
		}
		for j := 0; j < len(v); j++ {
			c := v[j]
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
				return nil, fmt.Errorf("%s[%d] is not hex", key, i)
			}
		}
		vals[i] = strings.ToLower(v)
	}
	return vals, nil
}

func parseStrings(key string, val interface{}) ([]string, error) {
	arr, ok := val.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array of strings", key)
	}
	out := make([]string, len(arr))
	for i, v := range arr {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s must be an array of strings", key)
		}
		out[i] = s
	}
	return out, nil
}

func parseKinds(val interface{}) ([]int, error) {
	arr, ok := val.([]interface{})
	if !ok {
		return nil, fmt.Errorf("kinds must be an array of integers")
	}
	// No upper bound: a kind above 65535 can't exist, so it just matches
	// nothing — clients use e.g. 99999 as a "give me EOSE only" probe.
	out := make([]int, len(arr))
	for i, v := range arr {
		n, ok := v.(float64)
		if !ok || n != math.Trunc(n) || n < 0 || n > math.MaxInt32 {
			return nil, fmt.Errorf("kinds[%d] must be a non-negative integer", i)
		}
		out[i] = int(n)
	}
	return out, nil
}

// parseNonNegative reads a JSON number that must be >= 0. Fractions are
// truncated rather than rejected: some clients send since/until straight from
// Date.now()/1000.
func parseNonNegative(key string, val interface{}) (int64, error) {
	n, ok := val.(float64)
	if !ok || math.IsNaN(n) || n < 0 || n >= 1<<62 {
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}
	return int64(n), nil
}
