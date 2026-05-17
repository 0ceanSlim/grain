// NIP-86 (Relay Management API) dispatch lives here. The spec — see
// https://github.com/nostr-protocol/nips/blob/master/86.md — wraps a
// JSON-RPC-style envelope around a single HTTP endpoint:
//
//	POST / HTTP/1.1
//	Content-Type: application/nostr+json+rpc
//	Authorization: Nostr <base64-encoded kind-27235 event>
//
//	{"method": "<name>", "params": [...]}
//
// The response is `{"result": <method-specific>, "error": "<string>"}`,
// where `error` is empty on success. Every method requires NIP-98 auth
// and (in grain's implementation) signer == relay owner per
// relay_metadata.json — RequireOwner is the gate.
//
// This file ships read-only methods. Mutation methods (banpubkey /
// allowpubkey / changerelay* / etc.) will land in a follow-up commit
// once the on-disk write path is in place; `supportedmethods`
// advertises only what's actually wired so well-behaved clients can
// feature-detect.
package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// NIP86ContentType is the media type NIP-86 reserves. The root
// dispatcher in server/startup.go uses this string to route requests
// here ahead of the WebSocket upgrade check.
const NIP86ContentType = "application/nostr+json+rpc"

// nip86Request is the JSON-RPC envelope. params is intentionally
// `[]any` — different methods take different argument shapes (some
// strings, some objects, some empty) and decoding stays per-method.
type nip86Request struct {
	Method string `json:"method"`
	Params []any  `json:"params"`
}

// nip86Response mirrors the spec: present `error` on failure (HTTP
// 200), present `result` on success. The empty-string convention for
// "no error" comes straight from NIP-86 — using a separate optional
// field would force every successful response to carry a stub error
// key.
type nip86Response struct {
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

// nip86PubkeyEntry is the shape NIP-86 specifies for the entries in
// listallowedpubkeys / listbannedpubkeys. `reason` is allowed to be
// empty; grain doesn't track per-pubkey reasons yet, so it always is.
type nip86PubkeyEntry struct {
	Pubkey string `json:"pubkey"`
	Reason string `json:"reason,omitempty"`
}

// nip86IPEntry is the shape NIP-86 specifies for listblockedips.
type nip86IPEntry struct {
	IP     string `json:"ip"`
	Reason string `json:"reason,omitempty"`
}

// HandleNIP86 is the JSON-RPC entry point. All paths return HTTP 200
// with a JSON body — method errors live in the response envelope, not
// the HTTP status. Auth failures (no header, bad signature, non-owner)
// short-circuit via RequireOwner before any JSON-RPC parsing happens
// and DO use the right status codes (401 / 403) because they're not
// JSON-RPC errors — they're HTTP-level access control.
//
// @Summary      NIP-86 relay management
// @Description  JSON-RPC over a single POST endpoint per [NIP-86](https://github.com/nostr-protocol/nips/blob/master/86.md). Requires NIP-98 HTTP Auth and the signer must equal the relay owner pubkey in `relay_metadata.json`. This phase implements the read-only methods only — see `supportedmethods` for the live list. Body is `{"method": "<name>", "params": [...]}`; response is `{"result": ..., "error": ""}`.
// @Tags         nip86
// @Accept       json
// @Produce      json
// @Param        body  body      nip86Request  true  "JSON-RPC envelope"
// @Success      200   {object}  nip86Response
// @Failure      401   {string}  string         "Unauthorized — missing/bad NIP-98 header"
// @Failure      403   {string}  string         "Forbidden — signer is not relay owner"
// @Security     NostrAuth
// @Router       / [post]
func HandleNIP86(w http.ResponseWriter, r *http.Request) {
	// RequireOwner reads the body for the NIP-98 payload hash and
	// restores r.Body on the way out, so the JSON decode below sees
	// the same bytes the signer hashed.
	signer, ok := RequireOwner(w, r)
	if !ok {
		return
	}

	var req nip86Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.RelayAPI().Info("NIP-86 request body parse failed",
			"client_ip", utils.GetClientIP(r),
			"signer", signer,
			"error", err)
		writeNIP86Error(w, "invalid JSON-RPC request: "+err.Error())
		return
	}

	log.RelayAPI().Info("NIP-86 method invoked",
		"client_ip", utils.GetClientIP(r),
		"signer", signer,
		"method", req.Method)

	result, err := dispatchNIP86(req, signer)
	if err != "" {
		writeNIP86Error(w, err)
		return
	}
	writeNIP86Result(w, result)
}

// dispatchNIP86 routes a parsed request to the right handler.
// Returns (result, errorString) — the empty error string signals
// success, matching the envelope semantics. Unknown methods return
// a structured error rather than an HTTP code so clients can
// feature-detect via `supportedmethods` and a subsequent unknown-
// method response without parsing two different error shapes.
//
// `signer` is the relay-owner pubkey RequireOwner returned for this
// request; write methods log it for audit so admin actions can be
// traced even though grain doesn't have a structured audit log yet.
func dispatchNIP86(req nip86Request, signer string) (any, string) {
	switch req.Method {

	// ─── reads ─────────────────────────────────────────────────
	case "supportedmethods":
		return supportedNIP86Methods(), ""
	case "listallowedpubkeys":
		return listAllowedPubkeysNIP86(), ""
	case "listbannedpubkeys":
		return listBannedPubkeysNIP86(), ""
	case "listallowedkinds":
		return listAllowedKindsNIP86(), ""
	case "listblockedips":
		return listBlockedIPsNIP86(), ""

	// ─── pubkey writes ────────────────────────────────────────
	case "banpubkey":
		return runBanPubkey(req.Params, signer)
	case "unbanpubkey":
		return runUnbanPubkey(req.Params, signer)
	case "allowpubkey":
		return runAllowPubkey(req.Params, signer)
	case "unallowpubkey":
		return runUnallowPubkey(req.Params, signer)

	// ─── kind writes ──────────────────────────────────────────
	case "allowkind":
		return runAllowKind(req.Params, signer)
	case "disallowkind":
		return runDisallowKind(req.Params, signer)

	// ─── IP writes ────────────────────────────────────────────
	case "blockip":
		return runBlockIP(req.Params, signer)
	case "unblockip":
		return runUnblockIP(req.Params, signer)

	// ─── relay-metadata writes ────────────────────────────────
	case "changerelayname":
		return runChangeRelayMetadata(req.Params, signer, "name")
	case "changerelaydescription":
		return runChangeRelayMetadata(req.Params, signer, "description")
	case "changerelayicon":
		return runChangeRelayMetadata(req.Params, signer, "icon")

	// ─── grain_* phase 2: config section updates ─────────────
	// Each takes the full section blob as params[0] and stages
	// it (writes the YAML, suppresses fsnotify). Response carries
	// restart_pending: true; the dashboard issues grain_reloadconfig
	// when the operator clicks Apply.
	case "grain_updateserver":
		return runUpdateServer(req.Params, signer)
	case "grain_updateratelimit":
		return runUpdateRateLimit(req.Params, signer)
	case "grain_updateeventpurge":
		return runUpdateEventPurge(req.Params, signer)
	case "grain_updatelogging":
		return runUpdateLogging(req.Params, signer)
	case "grain_updateauth":
		return runUpdateAuth(req.Params, signer)
	case "grain_updatebackuprelay":
		return runUpdateBackupRelay(req.Params, signer)
	case "grain_updateresourcelimits":
		return runUpdateResourceLimits(req.Params, signer)
	case "grain_updateeventtimeconstraints":
		return runUpdateEventTimeConstraints(req.Params, signer)
	case "grain_updatewhitelistconfig":
		return runUpdateWhitelistConfig(req.Params, signer)
	case "grain_updateblacklistconfig":
		return runUpdateBlacklistConfig(req.Params, signer)

	// ─── grain_* phase 2: operational ────────────────────────
	case "grain_reloadconfig":
		return runReloadConfig(signer)
	case "grain_refreshcache":
		return runRefreshCache(signer)

	// ─── grain_* phase 2: reads ──────────────────────────────
	case "grain_whitelistconfig":
		return runGetWhitelistConfig()
	case "grain_blacklistconfig":
		return runGetBlacklistConfig()
	case "grain_stats_overview":
		return gatherStatsOverview(), ""

	default:
		return nil, "method not supported: " + req.Method
	}
}

// supportedNIP86Methods returns the methods this build actually
// implements. Spec calls this method out specifically so clients can
// feature-detect; we treat it as the source of truth and update it in
// lockstep with new wiring. Event-moderation methods
// (listeventsneedingmoderation / allowevent / banevent /
// listbannedevents) are deliberately excluded — they need a
// moderation queue grain doesn't have yet.
func supportedNIP86Methods() []string {
	return []string{
		// reads
		"supportedmethods",
		"listallowedpubkeys",
		"listbannedpubkeys",
		"listallowedkinds",
		"listblockedips",
		// writes
		"banpubkey",
		"unbanpubkey",
		"allowpubkey",
		"unallowpubkey",
		"allowkind",
		"disallowkind",
		"blockip",
		"unblockip",
		"changerelayname",
		"changerelaydescription",
		"changerelayicon",
		// grain extensions
		"grain_updateserver",
		"grain_updateratelimit",
		"grain_updateeventpurge",
		"grain_updatelogging",
		"grain_updateauth",
		"grain_updatebackuprelay",
		"grain_updateresourcelimits",
		"grain_updateeventtimeconstraints",
		"grain_updatewhitelistconfig",
		"grain_updateblacklistconfig",
		"grain_reloadconfig",
		"grain_refreshcache",
		"grain_whitelistconfig",
		"grain_blacklistconfig",
		"grain_stats_overview",
	}
}

// listAllowedPubkeysNIP86 returns grain's *configured* whitelist — the
// elevated-users registry, not the runtime enforcement state. When
// `pubkey_whitelist.enabled = false` the relay accepts events from
// everyone, but this method still returns only the configured set.
// Same data the REST endpoint /api/v1/relay/keys/whitelist exposes,
// reshaped to NIP-86's `{pubkey, reason}` array.
func listAllowedPubkeysNIP86() []nip86PubkeyEntry {
	cache := config.GetPubkeyCache()
	if cache == nil {
		return []nip86PubkeyEntry{}
	}
	pubkeys := cache.GetDirectWhitelistedPubkeys()
	out := make([]nip86PubkeyEntry, 0, len(pubkeys))
	for _, p := range pubkeys {
		out = append(out, nip86PubkeyEntry{Pubkey: p})
	}
	return out
}

// listBannedPubkeysNIP86 returns the union of permanent blacklist
// pubkeys and mutelist-derived pubkeys, same as the REST blacklist
// endpoint's cached source. Temporary bans are excluded by design —
// they age out on their own and aren't a stable answer to "list
// banned pubkeys".
func listBannedPubkeysNIP86() []nip86PubkeyEntry {
	cache := config.GetPubkeyCache()
	if cache == nil {
		return []nip86PubkeyEntry{}
	}
	pubkeys := cache.GetBlacklistedPubkeys()
	out := make([]nip86PubkeyEntry, 0, len(pubkeys))
	for _, p := range pubkeys {
		out = append(out, nip86PubkeyEntry{Pubkey: p})
	}
	return out
}

// listAllowedKindsNIP86 returns the configured kind whitelist. Like
// the pubkey list this is the registry, not the gate — present
// regardless of `kind_whitelist.enabled`. Kinds are stored as strings
// in YAML to support kind ranges/labels in the future; we parse to
// int here per spec and silently skip anything non-numeric so a
// malformed kinds entry doesn't break the whole call.
func listAllowedKindsNIP86() []int {
	cfg := config.GetWhitelistConfig()
	if cfg == nil {
		return []int{}
	}
	out := make([]int, 0, len(cfg.KindWhitelist.Kinds))
	for _, k := range cfg.KindWhitelist.Kinds {
		n, err := strconv.Atoi(k)
		if err != nil {
			log.RelayAPI().Warn("Skipping non-integer kind in whitelist",
				"raw", k,
				"error", err)
			continue
		}
		out = append(out, n)
	}
	return out
}

// listBlockedIPsNIP86 returns the merged permanent IP blocklist from
// config + sidecar. The reason field is left blank — grain tracks the
// reason internally (admin-curated vs auto-escalated) but doesn't
// expose that distinction over the API. Temp bans are excluded; they
// expire on their own.
func listBlockedIPsNIP86() []nip86IPEntry {
	ips := config.GetBlockedIPs()
	out := make([]nip86IPEntry, 0, len(ips))
	for _, ip := range ips {
		out = append(out, nip86IPEntry{IP: ip})
	}
	return out
}

// writeNIP86Result is the success envelope. Centralised so every
// method gets the same content-type and CORS treatment without
// repeating boilerplate.
func writeNIP86Result(w http.ResponseWriter, result any) {
	writeNIP86Response(w, nip86Response{Result: result})
}

// writeNIP86Error is the failure envelope. Always HTTP 200 — JSON-RPC
// errors are not HTTP errors. Reserve HTTP error codes for actual
// transport-level issues (auth, malformed request body).
func writeNIP86Error(w http.ResponseWriter, msg string) {
	writeNIP86Response(w, nip86Response{Error: msg})
}

func writeNIP86Response(w http.ResponseWriter, resp nip86Response) {
	w.Header().Set("Content-Type", NIP86ContentType)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.RelayAPI().Error("Failed to encode NIP-86 response", "error", err)
	}
}
