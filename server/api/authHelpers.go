// Package api hosts the relay-management HTTP handlers. authHelpers
// is the NIP-98 glue: it pulls a signed kind-27235 event out of the
// Authorization header, hashes the request body, hands both off to
// handlers.VerifyNIP98Event, and (for admin endpoints) checks that
// the authenticated pubkey matches the relay owner in
// relay_metadata.json.
//
// Per NIP-98 §Encoding the auth header is `Authorization: Nostr <b64>`
// where <b64> is the base64-encoded JSON of the event.
package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/0ceanslim/grain/server/handlers"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
	"github.com/0ceanslim/grain/server/utils/log"
)

// authzScheme is the HTTP authentication scheme reserved by NIP-98.
// It is matched case-insensitively per RFC 7235 §2.1.
const authzScheme = "nostr"

// maxAuthBodyBytes caps the size of an admin request body that we
// will buffer into memory in order to compute its sha256 hash. The
// relay management API takes JSON payloads — config blobs, pubkey
// lists — that are small by design. Past this cap we refuse the
// request rather than risk a memory-exhaustion vector against an
// authenticated endpoint.
const maxAuthBodyBytes = 1 << 20 // 1 MiB

// ErrMissingAuthHeader is returned when the Authorization header is
// not present at all. Callers convert this to a 401 with a
// WWW-Authenticate prompt so clients know to retry with credentials.
var ErrMissingAuthHeader = errors.New("missing Authorization header")

// ExtractNIP98Event pulls the signed event out of an HTTP request's
// Authorization header. Returns ErrMissingAuthHeader if the header
// is absent so callers can distinguish "no creds" (401, send
// challenge) from "bad creds" (401, but no point re-prompting with
// the same scheme).
func ExtractNIP98Event(r *http.Request) (nostr.Event, error) {
	hdr := r.Header.Get("Authorization")
	if hdr == "" {
		return nostr.Event{}, ErrMissingAuthHeader
	}
	// Header form: "Nostr <base64-event-json>". The scheme name is
	// case-insensitive; the payload is not.
	parts := strings.SplitN(hdr, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], authzScheme) {
		return nostr.Event{}, errors.New("Authorization header must use the Nostr scheme")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(parts[1]))
	if err != nil {
		return nostr.Event{}, fmt.Errorf("invalid base64 in Authorization header: %w", err)
	}
	var evt nostr.Event
	if err := json.Unmarshal(raw, &evt); err != nil {
		return nostr.Event{}, fmt.Errorf("invalid JSON in Authorization header: %w", err)
	}
	return evt, nil
}

// VerifyAPIAuth verifies the NIP-98 Authorization header on r and
// returns the authenticated pubkey. It reads the request body (if
// any) to compute the payload hash that NIP-98 requires; the body
// is restored on r.Body so downstream handlers can read it normally.
//
// Callers that need owner-only access should use RequireOwner, which
// wraps this with a relay_metadata.json owner check.
func VerifyAPIAuth(r *http.Request) (string, error) {
	evt, err := ExtractNIP98Event(r)
	if err != nil {
		return "", err
	}

	absURL := absoluteRequestURL(r)
	method := strings.ToUpper(r.Method)

	// Reading and re-buffering the body is the price NIP-98 charges
	// for binding the signature to the request payload. We only do
	// it when a body could exist; for GET/DELETE/HEAD with no body
	// the empty hash signals "no payload tag expected".
	bodyHash, err := hashAndRestoreBody(r)
	if err != nil {
		return "", err
	}

	if err := handlers.VerifyNIP98Event(evt, method, absURL, bodyHash); err != nil {
		return "", err
	}
	return evt.PubKey, nil
}

// IsRelayOwner reports whether the given pubkey is the relay owner
// recorded in relay_metadata.json. The comparison is case-insensitive
// because some clients/tools normalize hex differently; the on-disk
// metadata is the source of truth.
func IsRelayOwner(pubkey string) bool {
	owner := utils.GetRelayOwnerPubkey()
	if owner == "" {
		return false
	}
	return strings.EqualFold(owner, pubkey)
}

// RequireOwner is the gate for admin/management endpoints: it
// authenticates the request via NIP-98 and confirms the signer is the
// relay owner. On failure it writes the appropriate response and
// returns ok=false so the handler can simply early-return.
func RequireOwner(w http.ResponseWriter, r *http.Request) (string, bool) {
	pubkey, err := VerifyAPIAuth(r)
	if err != nil {
		clientIP := utils.GetClientIP(r)
		// 401 with a NIP-98 prompt for any auth-shaped failure so
		// well-behaved clients know to retry with credentials. We
		// log at Info because admin endpoints are low-volume and a
		// failure here is operationally interesting.
		log.RelayAPI().Info("NIP-98 auth failed",
			"client_ip", clientIP,
			"method", r.Method,
			"path", r.URL.Path,
			"error", err)
		w.Header().Set("WWW-Authenticate", "Nostr")
		http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
		return "", false
	}
	if !IsRelayOwner(pubkey) {
		log.RelayAPI().Warn("NIP-98 auth rejected: signer is not relay owner",
			"client_ip", utils.GetClientIP(r),
			"signer", pubkey,
			"method", r.Method,
			"path", r.URL.Path)
		http.Error(w, "Forbidden: signer is not relay owner", http.StatusForbidden)
		return "", false
	}
	return pubkey, true
}

// hashAndRestoreBody reads the request body, computes its sha256 hex,
// and rewinds r.Body so the downstream handler can still read it.
// Returns the empty string when there is no body (in which case the
// caller passes that through to VerifyNIP98Event, which then requires
// the payload tag to be absent).
//
// Bodies larger than maxAuthBodyBytes are rejected outright; admin
// JSON payloads have no business being multi-megabyte and the cap
// keeps an authenticated DoS off the table.
func hashAndRestoreBody(r *http.Request) (string, error) {
	if r.Body == nil || r.Body == http.NoBody {
		return "", nil
	}
	// Cap with a +1 sentinel so we can distinguish "exactly the
	// limit" (allowed) from "over the limit" (rejected).
	limited := io.LimitReader(r.Body, maxAuthBodyBytes+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("failed to read request body: %w", err)
	}
	// Always restore *something* even on error paths so callers
	// downstream don't trip over a nil body.
	r.Body = io.NopCloser(bytes.NewReader(buf))
	if int64(len(buf)) > maxAuthBodyBytes {
		return "", fmt.Errorf("request body exceeds %d bytes", maxAuthBodyBytes)
	}
	if len(buf) == 0 {
		return "", nil
	}
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:]), nil
}

// absoluteRequestURL reconstructs the absolute URL the client saw,
// honoring the standard reverse-proxy hints. NIP-98 binds the
// signature to this URL, so a relay sitting behind nginx/traefik
// MUST verify against the original (https://relay.example.com/...)
// rather than the proxied (http://127.0.0.1:8080/...) form, or every
// real-world client signature would fail.
func absoluteRequestURL(r *http.Request) string {
	scheme := "http"
	if proto := firstHeaderValue(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		scheme = strings.ToLower(proto)
	} else if r.TLS != nil {
		scheme = "https"
	} else if r.URL.Scheme != "" {
		scheme = r.URL.Scheme
	}
	host := r.Host
	if h := firstHeaderValue(r.Header.Get("X-Forwarded-Host")); h != "" {
		host = h
	}
	path := r.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	if r.URL.RawQuery != "" {
		return scheme + "://" + host + path + "?" + r.URL.RawQuery
	}
	return scheme + "://" + host + path
}

// firstHeaderValue returns the first comma-separated value of a
// header. X-Forwarded-* headers can be chains (proxy1, proxy2); the
// originating value is the leftmost.
func firstHeaderValue(v string) string {
	if v == "" {
		return ""
	}
	if i := strings.IndexByte(v, ','); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}
