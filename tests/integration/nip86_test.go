package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// The grain-nip86 docker scenario (port 8190) mounts:
//   - whitelist.yml with pubkey_whitelist.enabled=false and one
//     allowed pubkey derived from NIP86AllowedSeed
//   - blacklist.yml with one permanent banned pubkey from
//     NIP86BannedSeed and two IP blocks (203.0.113.7, 198.51.100.0/24)
//   - relay_metadata.json whose `pubkey` is derived from NIP86OwnerSeed
//
// Everything below signs requests as the owner unless the test name
// explicitly says otherwise.

const nip86URL = "http://127.0.0.1:8190/"

// nip86Request matches the dispatcher's envelope (see server/api/nip86.go).
type nip86Envelope struct {
	Method string `json:"method"`
	Params []any  `json:"params"`
}

type nip86Reply struct {
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
}

// callNIP86 builds, signs, and posts a NIP-86 request as the given
// keypair. Returns the HTTP status and (when status is 200) the parsed
// envelope. Body errors and non-200 responses surface as a status with
// a nil envelope so callers can assert on the auth/owner branches.
func callNIP86(t *testing.T, kp *tests.TestKeypair, method string, params []any) (int, *nip86Reply) {
	t.Helper()

	body, _ := json.Marshal(nip86Envelope{Method: method, Params: params})
	sum := sha256.Sum256(body)
	payloadHash := hex.EncodeToString(sum[:])

	tags := [][]string{
		{"u", nip86URL},
		{"method", "POST"},
		{"payload", payloadHash},
	}
	authEvt := kp.SignEvent(27235, "", tags)
	authBytes, _ := json.Marshal(authEvt)
	authHeader := "Nostr " + base64.StdEncoding.EncodeToString(authBytes)

	req, err := http.NewRequest(http.MethodPost, nip86URL, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/nostr+json+rpc")
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST nip86: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil
	}
	respBytes, _ := io.ReadAll(resp.Body)
	var env nip86Reply
	if err := json.Unmarshal(respBytes, &env); err != nil {
		t.Fatalf("decode reply: %v (body=%q)", err, respBytes)
	}
	return resp.StatusCode, &env
}

func TestNIP86_MissingAuthReturns401(t *testing.T) {
	body, _ := json.Marshal(nip86Envelope{Method: "supportedmethods"})
	req, _ := http.NewRequest(http.MethodPost, nip86URL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/nostr+json+rpc")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("WWW-Authenticate"); !strings.EqualFold(got, "Nostr") {
		t.Fatalf("expected WWW-Authenticate: Nostr, got %q", got)
	}
}

func TestNIP86_NonOwnerReturns403(t *testing.T) {
	// Allowed pubkey is not the owner — its NIP-98 event verifies
	// fine but it should be rejected by the owner check.
	kp := tests.NewDeterministicKeypair(tests.NIP86AllowedSeed)
	status, _ := callNIP86(t, kp, "supportedmethods", nil)
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", status)
	}
}

func TestNIP86_SupportedMethods(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	status, env := callNIP86(t, owner, "supportedmethods", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if env.Error != "" {
		t.Fatalf("expected no error, got %q", env.Error)
	}
	var methods []string
	if err := json.Unmarshal(env.Result, &methods); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	for _, want := range []string{"supportedmethods", "listallowedpubkeys", "listbannedpubkeys", "listallowedkinds", "listblockedips"} {
		if !containsStr(methods, want) {
			t.Fatalf("supportedmethods missing %q (got %v)", want, methods)
		}
	}
}

func TestNIP86_UnknownMethodReturnsEnvelopeError(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	status, env := callNIP86(t, owner, "definitelynotamethod", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200 (envelope error, not HTTP error), got %d", status)
	}
	if env.Error == "" {
		t.Fatalf("expected envelope error, got result %s", string(env.Result))
	}
}

func TestNIP86_ListAllowedPubkeys_RegistryNotGate(t *testing.T) {
	// Whitelist is configured but `enabled: false` — the registry
	// must still come back. Verifies the "elevated users registry,
	// not enforcement state" semantics.
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	allowed := tests.NewDeterministicKeypair(tests.NIP86AllowedSeed)

	status, env := callNIP86(t, owner, "listallowedpubkeys", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if env.Error != "" {
		t.Fatalf("expected no error, got %q", env.Error)
	}
	var entries []struct {
		Pubkey string `json:"pubkey"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(env.Result, &entries); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !pubkeyInEntries(entries, allowed.PubKey) {
		t.Fatalf("expected allowed pubkey %s in registry, got %+v", allowed.PubKey, entries)
	}
}

func TestNIP86_ListBannedPubkeys(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	banned := tests.NewDeterministicKeypair(tests.NIP86BannedSeed)

	status, env := callNIP86(t, owner, "listbannedpubkeys", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if env.Error != "" {
		t.Fatalf("expected no error, got %q", env.Error)
	}
	var entries []struct {
		Pubkey string `json:"pubkey"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(env.Result, &entries); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !pubkeyInEntries(entries, banned.PubKey) {
		t.Fatalf("expected banned pubkey %s in list, got %+v", banned.PubKey, entries)
	}
}

func TestNIP86_ListAllowedKinds(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	status, env := callNIP86(t, owner, "listallowedkinds", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if env.Error != "" {
		t.Fatalf("expected no error, got %q", env.Error)
	}
	var kinds []int
	if err := json.Unmarshal(env.Result, &kinds); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	// nip86-whitelist.yml configures kinds 1 and 30023.
	if !containsInt(kinds, 1) || !containsInt(kinds, 30023) {
		t.Fatalf("expected kinds [1 30023] (any order), got %v", kinds)
	}
}

func TestNIP86_ListBlockedIPs(t *testing.T) {
	owner := tests.NewDeterministicKeypair(tests.NIP86OwnerSeed)
	status, env := callNIP86(t, owner, "listblockedips", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if env.Error != "" {
		t.Fatalf("expected no error, got %q", env.Error)
	}
	var entries []struct {
		IP     string `json:"ip"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(env.Result, &entries); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	// Bare IP (rendered without /32) and a CIDR — both from
	// nip86-blacklist.yml's permanent_blocked_ips.
	if !ipInEntries(entries, "203.0.113.7") {
		t.Fatalf("expected 203.0.113.7 in blocked list, got %+v", entries)
	}
	if !ipInEntries(entries, "198.51.100.0/24") {
		t.Fatalf("expected 198.51.100.0/24 in blocked list, got %+v", entries)
	}
}

func containsStr(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func containsInt(s []int, want int) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}

func pubkeyInEntries(entries []struct {
	Pubkey string `json:"pubkey"`
	Reason string `json:"reason"`
}, want string) bool {
	for _, e := range entries {
		if strings.EqualFold(e.Pubkey, want) {
			return true
		}
	}
	return false
}

func ipInEntries(entries []struct {
	IP     string `json:"ip"`
	Reason string `json:"reason"`
}, want string) bool {
	for _, e := range entries {
		if e.IP == want {
			return true
		}
	}
	return false
}
