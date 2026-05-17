package integration

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/0ceanslim/grain/tests"
)

// Write-method tests for NIP-86 phase 1. Each test:
//
//   1. Calls a write method as the relay owner.
//   2. Asserts `result: true, error: ""` came back.
//   3. Calls the corresponding read method and asserts the new
//      state is visible (proves the watcher-suppression + cache
//      refresh path works — without those, the cache stays stale
//      and the assertion fails).
//   4. Cleans up by inverting the write in t.Cleanup so tests can
//      run in any order without poisoning the fixture state.

// helper assertions ──────────────────────────────────────────────

func assertResultTrue(t *testing.T, env *nip86Reply) {
	t.Helper()
	if env == nil {
		t.Fatalf("nil envelope (non-200 response or HTTP error)")
	}
	if env.Error != "" {
		t.Fatalf("expected no error, got %q", env.Error)
	}
	var b bool
	if err := json.Unmarshal(env.Result, &b); err != nil || !b {
		t.Fatalf("expected result:true, got %s", string(env.Result))
	}
}

func assertEnvelopeError(t *testing.T, env *nip86Reply, wantSubstr string) {
	t.Helper()
	if env == nil {
		t.Fatalf("nil envelope (non-200 response or HTTP error)")
	}
	if env.Error == "" {
		t.Fatalf("expected envelope error containing %q, got result %s", wantSubstr, string(env.Result))
	}
	if wantSubstr != "" && !contains(env.Error, wantSubstr) {
		t.Fatalf("error %q did not contain %q", env.Error, wantSubstr)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// pubkeyWrites ───────────────────────────────────────────────────

// TestNIP86_BanPubkey: ban a fresh pubkey, confirm it shows up in
// listbannedpubkeys, then unban it for isolation.
func TestNIP86_BanPubkey(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	target := tests.NewTestKeypair().PubKey

	status, env := callNIP86(t, owner, "banpubkey", []any{target, "test"})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	assertResultTrue(t, env)
	t.Cleanup(func() {
		callNIP86(t, owner, "unbanpubkey", []any{target, "cleanup"})
	})

	_, env = callNIP86(t, owner, "listbannedpubkeys", nil)
	var entries []struct {
		Pubkey string `json:"pubkey"`
	}
	_ = json.Unmarshal(env.Result, &entries)
	if !pubkeyIn(entries, target) {
		t.Fatalf("after ban, listbannedpubkeys did not include %s", target)
	}
}

func TestNIP86_UnbanPubkey(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	target := tests.NewTestKeypair().PubKey

	callNIP86(t, owner, "banpubkey", []any{target, "setup"})
	_, env := callNIP86(t, owner, "unbanpubkey", []any{target, "test"})
	assertResultTrue(t, env)

	_, env = callNIP86(t, owner, "listbannedpubkeys", nil)
	var entries []struct {
		Pubkey string `json:"pubkey"`
	}
	_ = json.Unmarshal(env.Result, &entries)
	if pubkeyIn(entries, target) {
		t.Fatalf("after unban, listbannedpubkeys still included %s", target)
	}
}

func TestNIP86_AllowPubkey(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	target := tests.NewTestKeypair().PubKey

	_, env := callNIP86(t, owner, "allowpubkey", []any{target, "test"})
	assertResultTrue(t, env)
	t.Cleanup(func() {
		callNIP86(t, owner, "unallowpubkey", []any{target, "cleanup"})
	})

	_, env = callNIP86(t, owner, "listallowedpubkeys", nil)
	var entries []struct {
		Pubkey string `json:"pubkey"`
	}
	_ = json.Unmarshal(env.Result, &entries)
	if !pubkeyIn(entries, target) {
		t.Fatalf("after allow, listallowedpubkeys did not include %s", target)
	}
}

func TestNIP86_UnallowPubkey(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	target := tests.NewTestKeypair().PubKey

	callNIP86(t, owner, "allowpubkey", []any{target, "setup"})
	_, env := callNIP86(t, owner, "unallowpubkey", []any{target, "test"})
	assertResultTrue(t, env)

	_, env = callNIP86(t, owner, "listallowedpubkeys", nil)
	var entries []struct {
		Pubkey string `json:"pubkey"`
	}
	_ = json.Unmarshal(env.Result, &entries)
	if pubkeyIn(entries, target) {
		t.Fatalf("after unallow, listallowedpubkeys still included %s", target)
	}
}

// kindWrites ─────────────────────────────────────────────────────

func TestNIP86_AllowKind(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	// Pick a kind unlikely to clash with fixture data (1, 30023).
	const target = 9999

	_, env := callNIP86(t, owner, "allowkind", []any{target})
	assertResultTrue(t, env)
	t.Cleanup(func() {
		callNIP86(t, owner, "disallowkind", []any{target})
	})

	_, env = callNIP86(t, owner, "listallowedkinds", nil)
	var kinds []int
	_ = json.Unmarshal(env.Result, &kinds)
	if !intIn(kinds, target) {
		t.Fatalf("after allowkind, listallowedkinds did not include %d (got %v)", target, kinds)
	}
}

func TestNIP86_DisallowKind(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	const target = 8888

	callNIP86(t, owner, "allowkind", []any{target})
	_, env := callNIP86(t, owner, "disallowkind", []any{target})
	assertResultTrue(t, env)

	_, env = callNIP86(t, owner, "listallowedkinds", nil)
	var kinds []int
	_ = json.Unmarshal(env.Result, &kinds)
	if intIn(kinds, target) {
		t.Fatalf("after disallowkind, listallowedkinds still included %d", target)
	}
}

// IP writes ──────────────────────────────────────────────────────

func TestNIP86_BlockIP(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	target := "192.0.2.42" // TEST-NET-1, never routable

	_, env := callNIP86(t, owner, "blockip", []any{target, "test"})
	assertResultTrue(t, env)
	t.Cleanup(func() {
		callNIP86(t, owner, "unblockip", []any{target, "cleanup"})
	})

	_, env = callNIP86(t, owner, "listblockedips", nil)
	var entries []struct {
		IP string `json:"ip"`
	}
	_ = json.Unmarshal(env.Result, &entries)
	if !ipInList(entries, target) {
		t.Fatalf("after blockip, listblockedips did not include %s (got %+v)", target, entries)
	}
}

func TestNIP86_UnblockIP(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	target := "192.0.2.43"

	callNIP86(t, owner, "blockip", []any{target, "setup"})
	_, env := callNIP86(t, owner, "unblockip", []any{target, "test"})
	assertResultTrue(t, env)

	_, env = callNIP86(t, owner, "listblockedips", nil)
	var entries []struct {
		IP string `json:"ip"`
	}
	_ = json.Unmarshal(env.Result, &entries)
	if ipInList(entries, target) {
		t.Fatalf("after unblockip, listblockedips still included %s", target)
	}
}

// relay-metadata writes ──────────────────────────────────────────

// Captures the NIP-11 info doc so the test can both assert on the
// new value and restore the original in cleanup.
func fetchRelayInfo(t *testing.T) map[string]any {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:8190/", nil)
	req.Header.Set("Accept", "application/nostr+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET nip11: %v", err)
	}
	defer resp.Body.Close()
	var info map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("decode nip11: %v", err)
	}
	return info
}

func TestNIP86_ChangeRelayName(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	before := fetchRelayInfo(t)
	orig, _ := before["name"].(string)
	target := "GRAIN test rename"

	_, env := callNIP86(t, owner, "changerelayname", []any{target})
	assertResultTrue(t, env)
	t.Cleanup(func() {
		callNIP86(t, owner, "changerelayname", []any{orig})
	})

	after := fetchRelayInfo(t)
	if got, _ := after["name"].(string); got != target {
		t.Fatalf("relay name not updated: got %q want %q", got, target)
	}
}

func TestNIP86_ChangeRelayDescription(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	before := fetchRelayInfo(t)
	orig, _ := before["description"].(string)
	target := "ephemeral test description"

	_, env := callNIP86(t, owner, "changerelaydescription", []any{target})
	assertResultTrue(t, env)
	t.Cleanup(func() {
		callNIP86(t, owner, "changerelaydescription", []any{orig})
	})

	after := fetchRelayInfo(t)
	if got, _ := after["description"].(string); got != target {
		t.Fatalf("relay description not updated: got %q want %q", got, target)
	}
}

func TestNIP86_ChangeRelayIcon(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	before := fetchRelayInfo(t)
	orig, _ := before["icon"].(string)
	target := "https://example.com/grain-test.png"

	_, env := callNIP86(t, owner, "changerelayicon", []any{target})
	assertResultTrue(t, env)
	t.Cleanup(func() {
		callNIP86(t, owner, "changerelayicon", []any{orig})
	})

	after := fetchRelayInfo(t)
	if got, _ := after["icon"].(string); got != target {
		t.Fatalf("relay icon not updated: got %q want %q", got, target)
	}
}

// concurrency + suppression ──────────────────────────────────────

// TestNIP86_ConcurrentBans drives 10 simultaneous banpubkey calls
// with distinct pubkeys. Without ConfigMu (the package-wide mutex
// new admin writes take), the read-modify-write on
// blacklistConfig.PermanentBlacklistPubkeys would race and lose
// some entries.
func TestNIP86_ConcurrentBans(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	targets := make([]string, 10)
	for i := range targets {
		targets[i] = tests.NewTestKeypair().PubKey
	}
	t.Cleanup(func() {
		for _, p := range targets {
			callNIP86(t, owner, "unbanpubkey", []any{p, "cleanup"})
		}
	})

	var wg sync.WaitGroup
	for _, p := range targets {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, env := callNIP86(t, owner, "banpubkey", []any{p, "concurrent"})
			if status != http.StatusOK || env.Error != "" {
				t.Errorf("concurrent ban of %s failed: status=%d err=%q", p, status, env.Error)
			}
		}()
	}
	wg.Wait()

	_, env := callNIP86(t, owner, "listbannedpubkeys", nil)
	var entries []struct {
		Pubkey string `json:"pubkey"`
	}
	_ = json.Unmarshal(env.Result, &entries)
	for _, p := range targets {
		if !pubkeyIn(entries, p) {
			t.Fatalf("after concurrent bans, listbannedpubkeys missing %s", p)
		}
	}
}

// TestNIP86_WriteVisibleImmediately is the practical
// suppression-works test: if the watcher had fired a restart on
// the write, the cache would be stale (or the connection would
// drop). Instead we ban + immediately list and assert the pubkey
// is there. Read happens via the same HTTP client + reused TCP
// connection that did the write — a restart would close it.
func TestNIP86_WriteVisibleImmediately(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	target := tests.NewTestKeypair().PubKey
	t.Cleanup(func() {
		callNIP86(t, owner, "unbanpubkey", []any{target, "cleanup"})
	})

	status, env := callNIP86(t, owner, "banpubkey", []any{target, "test"})
	if status != http.StatusOK || env.Error != "" {
		t.Fatalf("ban failed: status=%d err=%q", status, env.Error)
	}
	// No delay — if suppression works, the cache is already fresh.
	_, env = callNIP86(t, owner, "listbannedpubkeys", nil)
	var entries []struct {
		Pubkey string `json:"pubkey"`
	}
	_ = json.Unmarshal(env.Result, &entries)
	if !pubkeyIn(entries, target) {
		t.Fatalf("expected immediate visibility after ban")
	}
}

// invalid input ──────────────────────────────────────────────────

// TestNIP86_InvalidInput asserts that bad params surface as a
// JSON-RPC envelope error (HTTP 200 + non-empty `error`), never as
// an HTTP 5xx. Operators reading API responses from the dashboard
// shouldn't have to know the difference between transport errors
// and validation errors.
func TestNIP86_InvalidInput(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)

	cases := []struct {
		method string
		params []any
		want   string
	}{
		{"banpubkey", []any{"nothex"}, "invalid pubkey"},
		{"banpubkey", []any{}, "invalid pubkey"},
		{"allowpubkey", []any{"deadbeef"}, "invalid pubkey"},
		{"allowkind", []any{-1}, "invalid kind"},
		{"allowkind", []any{1_000_000}, "invalid kind"},
		{"allowkind", []any{}, "invalid kind"},
		{"blockip", []any{"not.an.ip.addr"}, "invalid"},
		{"blockip", []any{}, "invalid ip"},
		{"changerelayicon", []any{"javascript:alert(1)"}, "invalid icon"},
		{"changerelayname", []any{}, "missing value"},
		{"changerelayname", []any{""}, "name cannot be empty"},
	}
	for _, c := range cases {
		status, env := callNIP86(t, owner, c.method, c.params)
		if status != http.StatusOK {
			t.Fatalf("%s: expected 200 envelope, got HTTP %d", c.method, status)
		}
		assertEnvelopeError(t, env, c.want)
	}
}

// list helpers ───────────────────────────────────────────────────

func pubkeyIn(entries []struct {
	Pubkey string `json:"pubkey"`
}, want string) bool {
	for _, e := range entries {
		if e.Pubkey == want {
			return true
		}
	}
	return false
}

func intIn(s []int, want int) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func ipInList(entries []struct {
	IP string `json:"ip"`
}, want string) bool {
	for _, e := range entries {
		if e.IP == want {
			return true
		}
	}
	return false
}
