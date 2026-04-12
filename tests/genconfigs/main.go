// genconfigs writes dynamic test fixtures that depend on deterministically
// derived keypairs (so test Go code and config YAML agree on the same pubkey).
//
// Run from the tests/ directory before `docker compose up`:
//
//	go run ./genconfigs
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
}
