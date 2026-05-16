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
	"regexp"
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

	// NIP-86 fixtures. The same line-rewrite trick that whitelist-rules
	// uses doesn't fit here because we're swapping single pubkey hex
	// strings; just regex over a 64-hex run and replace.
	ownerPub := DerivePubKeyHex(NIP86OwnerSeed)
	allowedPub := DerivePubKeyHex(NIP86AllowedSeed)
	bannedPub := DerivePubKeyHex(NIP86BannedSeed)
	fmt.Printf("nip86 owner pubkey:   %s\n", ownerPub)
	fmt.Printf("nip86 allowed pubkey: %s\n", allowedPub)
	fmt.Printf("nip86 banned pubkey:  %s\n", bannedPub)

	if err := replaceFirst64Hex("docker/configs/nip86-relay_metadata.json", ownerPub); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if err := replaceFirst64Hex("docker/configs/nip86-whitelist.yml", allowedPub); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	if err := replaceFirst64Hex("docker/configs/nip86-blacklist.yml", bannedPub); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// replaceFirst64Hex swaps the first 64-character hex run found in the
// file with the given replacement. Used to keep NIP-86 fixtures in
// sync with deterministic test keypairs without parsing each format.
// Idempotent: re-running with the same replacement is a no-op.
func replaceFirst64Hex(path, replacement string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	re := regexp.MustCompile(`[0-9a-fA-F]{64}`)
	loc := re.FindIndex(raw)
	if loc == nil {
		return fmt.Errorf("%s: no 64-hex placeholder found", path)
	}
	out := append([]byte{}, raw[:loc[0]]...)
	out = append(out, []byte(replacement)...)
	out = append(out, raw[loc[1]:]...)
	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("updated %s\n", path)
	return nil
}
