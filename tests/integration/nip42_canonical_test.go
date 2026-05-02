package integration

import (
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// Verifies the NIP-42 relay-URL comparison applies light normalization
// rather than strict string equality. The auth scenario relay has
// `auth.relay_url: ws://127.0.0.1:8186`; this test signs AUTH with
// `ws://127.0.0.1:8186/` (trailing slash) and expects success.
//
// Prior to the canonicalization fix, this AUTH was rejected with
// "invalid: relay URL does not match" — accounting for ~500 retry-
// spam events in production logs from clients that include the slash.

func TestNIP42_RelayURLTrailingSlashAccepted(t *testing.T) {
	kp := tests.NewTestKeypair()
	c := tests.NewTestClientAt(t, tests.AuthRelayURL)
	defer c.Close()

	withSlash := tests.AuthRelayURL + "/"
	ok, reason := c.PerformAuth(kp, withSlash, 3*time.Second)
	if !ok {
		t.Fatalf("AUTH with trailing-slash relay tag rejected: %q", reason)
	}
}
