// genconfigs writes dynamic test fixtures that depend on deterministically
// derived keypairs (so test Go code and config YAML agree on the same pubkey).
//
// Run from the tests/ directory before `docker compose up`:
//
//	go run ./genconfigs
//
// The NIP-86 fixtures are written from canonical templates on every run.
// They're bind-mounted read-write into the test container, and the relay's
// AtomicWriteFile falls back to an in-place write when rename-over-bind-mount
// fails (EBUSY) — so a `grain_update*` call during the suite writes re-emitted
// YAML/JSON straight back to the host file, losing the hand-maintained shape.
// Regenerating from templates here restores that shape every run, keeping
// `git status` clean regardless of what the tests mutated (issue #78).
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
)

// WhitelistSeed must match tests.WhitelistSeed in tests/helpers.go.
const WhitelistSeed = "grain-test-whitelist-allowed"

// NIP86OwnerSeed must match tests.NIP86OwnerSeed. Used to derive the
// pubkey planted into nip86-relay_metadata.json so tests/integration/
// nip86_test.go can sign requests as the relay owner.
const NIP86OwnerSeed = "grain-test-nip86-owner"

// NIP86AllowedSeed must match tests.NIP86AllowedSeed. Planted into
// nip86-whitelist.yml as the sole allowed pubkey so listallowedpubkeys
// has a deterministic answer.
const NIP86AllowedSeed = "grain-test-nip86-allowed"

// NIP86BannedSeed must match tests.NIP86BannedSeed. Planted into
// nip86-blacklist.yml so listbannedpubkeys has a deterministic answer.
const NIP86BannedSeed = "grain-test-nip86-banned"

// DerivePubKeyHex computes the x-only pubkey hex for a given seed.
// MUST match tests.NewDeterministicKeypair in tests/helpers.go.
func DerivePubKeyHex(seed string) string {
	h := sha256.Sum256([]byte(seed))
	priv, _ := btcec.PrivKeyFromBytes(h[:])
	return hex.EncodeToString(schnorr.SerializePubKey(priv.PubKey()))
}

func main() {
	whitelistPub := DerivePubKeyHex(WhitelistSeed)
	fmt.Printf("whitelist allowed pubkey: %s\n", whitelistPub)

	path := "docker/configs/whitelist-rules.yml"
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read %s: %v\n", path, err)
		os.Exit(1)
	}
	out := strings.ReplaceAll(string(raw), "__GENERATED__", whitelistPub)
	// Also replace any previously-generated hex (for idempotent re-runs).
	// Find the `pubkeys:` block and rewrite its first list entry.
	if !strings.Contains(out, whitelistPub) {
		// Rewrite placeholder-free version: replace everything between
		// "pubkeys:" and "npubs:" with our single pubkey entry.
		start := strings.Index(out, "pubkeys:")
		end := strings.Index(out, "npubs:")
		if start >= 0 && end > start {
			replacement := fmt.Sprintf("pubkeys:\n    - \"%s\"\n  ", whitelistPub)
			out = out[:start] + replacement + out[end:]
		}
	}
	if err := os.WriteFile(path, []byte(out), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("updated %s\n", path)

	// NIP-86 fixtures, written from canonical templates so a clobbered
	// fixture is restored to its hand-maintained shape on every run.
	ownerPub := DerivePubKeyHex(NIP86OwnerSeed)
	allowedPub := DerivePubKeyHex(NIP86AllowedSeed)
	bannedPub := DerivePubKeyHex(NIP86BannedSeed)
	fmt.Printf("nip86 owner pubkey:   %s\n", ownerPub)
	fmt.Printf("nip86 allowed pubkey: %s\n", allowedPub)
	fmt.Printf("nip86 banned pubkey:  %s\n", bannedPub)

	writeFixture("docker/configs/nip86.yml", nip86ConfigTmpl)
	writeFixture("docker/configs/nip86-relay_metadata.json", fmt.Sprintf(nip86MetadataTmpl, ownerPub))
	writeFixture("docker/configs/nip86-whitelist.yml", fmt.Sprintf(nip86WhitelistTmpl, allowedPub))
	writeFixture("docker/configs/nip86-blacklist.yml", fmt.Sprintf(nip86BlacklistTmpl, bannedPub))
}

// writeFixture writes a fully-rendered fixture to disk, exiting on error.
func writeFixture(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write %s: %v\n", path, err)
		os.Exit(1)
	}
	fmt.Printf("updated %s\n", path)
}

// nip86ConfigTmpl is the relay config the NIP-86 suite runs against. No
// derived values; templated only so grain_update* writes that mutate it
// during the suite get reset on the next run.
const nip86ConfigTmpl = `logging:
  level: "warn"
  file: "debug"
  max_log_size_mb: 10
  structure: false
  check_interval_min: 10
  backup_count: 2

client:
  index_relays: []

database:
  path: "data"
  map_size_mb: 32

server:
  port: :8190
  read_timeout: 60
  write_timeout: 20
  idle_timeout: 1200
  max_subscriptions_per_client: 10
  implicit_req_limit: 500

resource_limits:
  cpu_cores: 2
  memory_mb: 512
  heap_size_mb: 256

auth:
  required: false
  relay_url: "http://127.0.0.1:8190"

backup_relay:
  enabled: false
  url: ""

event_purge:
  enabled: false
  disable_at_startup: true
  keep_interval_hours: 24
  purge_interval_minutes: 240
  purge_by_category: {regular: false, replaceable: false, addressable: false, deprecated: false}
  purge_by_kind_enabled: false
  kinds_to_purge: []
  exclude_whitelisted: true

event_time_constraints:
  min_created_at: 1577836800
  max_created_at_string: now+5m

rate_limit:
  ws_limit: 500
  ws_burst: 1000
  event_limit: 500
  event_burst: 1000
  req_limit: 500
  req_burst: 1000
  max_event_size: 524288
  kind_size_limits: []
  category_limits: {}
  kind_limits: []

# Mirrored under ` + "`blacklist:`" + ` because LoadIPBlocklist reads
# ServerConfig.Blacklist (config.yml) rather than the standalone
# blacklist.yml. The pubkey-bans + words live in the sibling
# blacklist.yml file; IPs go here for the loader to find them.
blacklist:
  permanent_blocked_ips:
    - "203.0.113.7"
    - "198.51.100.0/24"
`

// nip86MetadataTmpl — %s is the relay-owner pubkey NIP-98 authenticates against.
const nip86MetadataTmpl = `{
  "_comment": "Generated by tests/genconfigs — do not edit by hand. The pubkey field is the relay owner identity NIP-98 authenticates against.",
  "name": "GRAIN NIP-86 test relay",
  "description": "Fixture for NIP-86 integration tests.",
  "pubkey": "%s",
  "contact": "",
  "supported_nips": [1, 11, 42, 86, 98],
  "software": "https://github.com/0ceanslim/grain",
  "version": "test"
}
`

// nip86WhitelistTmpl — %s is the sole allowed pubkey.
const nip86WhitelistTmpl = `# Generated by tests/genconfigs — do not edit by hand.
# Regenerate via: go run ./genconfigs (from tests/)
#
# Deliberately set ` + "`enabled: false`" + ` so the relay accepts events from
# anyone. NIP-86's listallowedpubkeys returns the registry regardless
# of the gate state, and the test verifies exactly that distinction.
pubkey_whitelist:
  enabled: false
  pubkeys:
    - "%s"   # replaced at test-start by genconfigs
  npubs: []
  cache_refresh_minutes: 60

kind_whitelist:
  enabled: false
  kinds:
    - "1"
    - "30023"
  cache_refresh_minutes: 60

domain_whitelist:
  enabled: false
  domains: []
  cache_refresh_minutes: 120
`

// nip86BlacklistTmpl — %s is the sole banned pubkey.
const nip86BlacklistTmpl = `# Generated by tests/genconfigs — do not edit by hand.
# Regenerate via: go run ./genconfigs (from tests/)
enabled: true

permanent_ban_words: []
temp_ban_words: []
max_temp_bans: 0
temp_ban_duration: 0

permanent_blacklist_pubkeys:
  - "%s"   # replaced at test-start by genconfigs

permanent_blacklist_npubs: []

mutelist_authors: []
mutelist_cache_refresh_minutes: 60

# permanent_blocked_ips lives in config.yml under ` + "`blacklist:`" + ` instead
# of here — LoadIPBlocklist reads ServerConfig.Blacklist, not the
# blacklist.yml that GetBlacklistConfig returns. See nip86.yml.
`
