// NIP-86 write-method handlers. The reads and the dispatcher live
// in server/api/nip86.go; the writes are split out here so the file
// stays readable as the method count grows. Each runX function:
//
//   1. Pulls + validates the JSON-RPC params (positional, per spec).
//   2. Calls the matching config helper.
//   3. Logs the action with the signer pubkey for audit.
//   4. Returns `(true, "")` on success or `(nil, "<msg>")` on failure.
//
// The signer pubkey is the relay-owner pubkey that already passed
// RequireOwner — every write is gated, so logging it is purely
// audit. Once grain has a structured audit log (out of scope for
// phase 1), these logs become the seed for that.

package api

import (
	"encoding/hex"
	"net/url"
	"strconv"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// ─── param extraction helpers ────────────────────────────────────

// paramString reads positional index `i` from params and returns it
// as a string. Returns (value, true) if present and a string,
// (zero, false) otherwise. Per NIP-86 every method uses positional
// params; we never read by name.
func paramString(params []any, i int) (string, bool) {
	if i >= len(params) {
		return "", false
	}
	s, ok := params[i].(string)
	return s, ok
}

// paramInt accepts either a JSON number (default float64 in
// encoding/json) or a string that parses to an integer. The spec is
// ambiguous on whether kinds come as numbers or strings; tolerating
// both means a client picking the safer string encoding still works.
func paramInt(params []any, i int) (int, bool) {
	if i >= len(params) {
		return 0, false
	}
	switch v := params[i].(type) {
	case float64:
		return int(v), v == float64(int(v))
	case string:
		n, err := strconv.Atoi(v)
		return n, err == nil
	default:
		return 0, false
	}
}

// ─── input validators ────────────────────────────────────────────

// isHexPubkey returns true for exactly 64 lower-case hex characters.
// We accept upper-case too because operators paste from various
// tools, but normalize callers to lowercase before storage.
func isHexPubkey(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// isReasonableKind keeps invalid kind values out of the whitelist.
// NIP-01 doesn't formally cap kinds but >65535 is well past every
// allocated kind range and almost certainly a typo or attack.
func isReasonableKind(n int) bool { return n >= 0 && n <= 65535 }

// isParseableURL accepts http(s) and data: URLs for icon. Other
// schemes (file:, javascript:) would be a security smell on the
// admin metadata.
func isParseableURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" {
		return false
	}
	switch u.Scheme {
	case "http", "https", "data":
		return true
	default:
		return false
	}
}

// ─── pubkey writes ───────────────────────────────────────────────

func runBanPubkey(params []any, signer string) (any, string) {
	pubkey, ok := paramString(params, 0)
	if !ok || !isHexPubkey(pubkey) {
		return nil, "invalid pubkey"
	}
	reason, _ := paramString(params, 1) // optional
	if err := config.AddToPermanentBlacklist(pubkey); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 banpubkey", "signer", signer, "pubkey", pubkey, "reason", reason)
	return true, ""
}

func runUnbanPubkey(params []any, signer string) (any, string) {
	pubkey, ok := paramString(params, 0)
	if !ok || !isHexPubkey(pubkey) {
		return nil, "invalid pubkey"
	}
	reason, _ := paramString(params, 1)
	if err := config.RemoveFromPermanentBlacklist(pubkey); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 unbanpubkey", "signer", signer, "pubkey", pubkey, "reason", reason)
	return true, ""
}

func runAllowPubkey(params []any, signer string) (any, string) {
	pubkey, ok := paramString(params, 0)
	if !ok || !isHexPubkey(pubkey) {
		return nil, "invalid pubkey"
	}
	reason, _ := paramString(params, 1)
	if err := config.AddPubkeyToWhitelist(pubkey); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 allowpubkey", "signer", signer, "pubkey", pubkey, "reason", reason)
	return true, ""
}

func runUnallowPubkey(params []any, signer string) (any, string) {
	pubkey, ok := paramString(params, 0)
	if !ok || !isHexPubkey(pubkey) {
		return nil, "invalid pubkey"
	}
	reason, _ := paramString(params, 1)
	if err := config.RemovePubkeyFromWhitelist(pubkey); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 unallowpubkey", "signer", signer, "pubkey", pubkey, "reason", reason)
	return true, ""
}

// ─── kind writes ─────────────────────────────────────────────────

func runAllowKind(params []any, signer string) (any, string) {
	kind, ok := paramInt(params, 0)
	if !ok || !isReasonableKind(kind) {
		return nil, "invalid kind"
	}
	if err := config.AddKindToWhitelist(kind); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 allowkind", "signer", signer, "kind", kind)
	return true, ""
}

func runDisallowKind(params []any, signer string) (any, string) {
	kind, ok := paramInt(params, 0)
	if !ok || !isReasonableKind(kind) {
		return nil, "invalid kind"
	}
	if err := config.RemoveKindFromWhitelist(kind); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 disallowkind", "signer", signer, "kind", kind)
	return true, ""
}

// ─── IP writes ───────────────────────────────────────────────────

func runBlockIP(params []any, signer string) (any, string) {
	ip, ok := paramString(params, 0)
	if !ok || ip == "" {
		return nil, "invalid ip"
	}
	reason, _ := paramString(params, 1)
	if err := config.AddAdminBlockedIP(ip); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 blockip", "signer", signer, "ip", ip, "reason", reason)
	return true, ""
}

func runUnblockIP(params []any, signer string) (any, string) {
	ip, ok := paramString(params, 0)
	if !ok || ip == "" {
		return nil, "invalid ip"
	}
	reason, _ := paramString(params, 1)
	if err := config.RemoveAdminBlockedIP(ip); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 unblockip", "signer", signer, "ip", ip, "reason", reason)
	return true, ""
}

// ─── relay metadata writes ───────────────────────────────────────

// runChangeRelayMetadata handles changerelayname / changerelay-
// description / changerelayicon as a single helper — they only
// differ in which field gets patched. The dispatcher passes the
// field name; this function pulls the value from params[0] and
// routes it to the right pointer arg of utils.UpdateRelayMetadata.
func runChangeRelayMetadata(params []any, signer, field string) (any, string) {
	value, ok := paramString(params, 0)
	if !ok {
		return nil, "missing value"
	}
	var name, description, icon, banner *string
	switch field {
	case "name":
		if value == "" {
			return nil, "name cannot be empty"
		}
		name = &value
	case "description":
		// Empty description is allowed; some operators clear it.
		description = &value
	case "icon":
		if value != "" && !isParseableURL(value) {
			return nil, "invalid icon URL"
		}
		icon = &value
	case "banner":
		if value != "" && !isParseableURL(value) {
			return nil, "invalid banner URL"
		}
		banner = &value
	default:
		return nil, "unknown relay-metadata field: " + field
	}
	if err := utils.UpdateRelayMetadata(name, description, icon, banner); err != nil {
		return nil, err.Error()
	}
	log.RelayAPI().Info("NIP-86 changerelay*", "signer", signer, "field", field, "value", value)
	return true, ""
}
