package utils

import (
	"encoding/json"
	"strings"
	"testing"
)

// The NIP-11 limitation block must not advertise a misleading "0" for caps the
// relay doesn't set — unset numeric fields are omitted, while the boolean
// fields (auth_required/payment_required/restricted_writes) always appear.
func TestRelayLimitationOmitsUnsetNumerics(t *testing.T) {
	lim := RelayLimitation{MaxMessageLength: 524288} // content/subs/limit left 0
	b, err := json.Marshal(lim)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)

	if !strings.Contains(s, `"max_message_length":524288`) {
		t.Errorf("a set cap should be present: %s", s)
	}
	for _, omitted := range []string{"max_content_length", "max_subscriptions", "max_limit", "created_at_lower_limit"} {
		if strings.Contains(s, omitted) {
			t.Errorf("unset %q should be omitted, got: %s", omitted, s)
		}
	}
	if !strings.Contains(s, `"auth_required":false`) {
		t.Errorf("bool fields should always be present: %s", s)
	}
}
