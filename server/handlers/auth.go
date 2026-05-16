package handlers

import (
	"encoding/json"
	"errors"
	"net/url"
	"sync"
	"time"

	"github.com/0ceanslim/grain/config"
	"github.com/0ceanslim/grain/server/handlers/response"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
	"github.com/0ceanslim/grain/server/utils/relayurl"
	"github.com/0ceanslim/grain/server/validation"
)

// NIP98AuthKind is the event kind reserved by NIP-98 for HTTP Auth.
const NIP98AuthKind = 27235

// nip98TimeWindow is the ±tolerance applied to NIP-98 `created_at`.
// The spec says implementations "should reject requests with a value
// in the past or future, with a reasonable margin." 60 seconds is the
// commonly cited reasonable margin and is what this implementation
// uses both backwards (replay window) and forwards (clock skew).
const nip98TimeWindow = 60 * time.Second

// Mutex to protect auth data
var authMu sync.Mutex

// Maps to track authentication sessions. authSessions stores the
// authenticated pubkey (hex) per connection; an empty/missing entry
// means the connection has not completed NIP-42 AUTH. We need the
// pubkey itself (not just a bool) so NIP-70 can verify a protected
// event came from its author's authenticated connection.
var challenges = make(map[nostr.ClientInterface]string)
var authSessions = make(map[nostr.ClientInterface]string)

// HandleAuth processes the "AUTH" message type as defined in NIP-42
func HandleAuth(client nostr.ClientInterface, message []interface{}) {
	if len(message) != 2 {
		log.Auth().Debug("Invalid AUTH message format")
		response.SendNotice(client, "", "Invalid AUTH message format")
		return
	}

	authData, ok := message[1].(map[string]interface{})
	if !ok {
		log.Auth().Debug("Invalid auth data format")
		response.SendNotice(client, "", "Invalid auth data format")
		return
	}

	authBytes, err := json.Marshal(authData)
	if err != nil {
		log.Auth().Error("Error marshaling auth data", "error", err)
		response.SendNotice(client, "", "Error marshaling auth data")
		return
	}

	var authEvent nostr.Event
	err = json.Unmarshal(authBytes, &authEvent)
	if err != nil {
		log.Auth().Error("Error unmarshaling auth data", "error", err)
		response.SendNotice(client, "", "Error unmarshaling auth data")
		return
	}

	err = VerifyAuthEvent(client, authEvent)
	if err != nil {
		// Pull the relay tag for the failure log so URL-mismatch
		// problems can be diagnosed from the file alone (otherwise
		// the only signal is the opaque "relay URL does not match").
		gotRelay, _ := extractTag(authEvent.Tags, "relay")
		log.Auth().Info("Auth verification failed",
			"event_id", authEvent.ID,
			"pubkey", authEvent.PubKey,
			"relay_tag", gotRelay,
			"error", err)
		response.SendOK(client, authEvent.ID, false, err.Error())
		return
	}

	// Mark the session as authenticated after successful verification
	SetAuthenticated(client, authEvent.PubKey)
	ClearChallengeForConnection(client) // Clear used challenge

	log.Auth().Info("Authentication successful", "pubkey", authEvent.PubKey)
	response.SendOK(client, authEvent.ID, true, "")
}

// VerifyAuthEvent verifies the authentication event according to NIP-42
func VerifyAuthEvent(client nostr.ClientInterface, evt nostr.Event) error {
	if evt.Kind != 22242 {
		return errors.New("invalid: event kind must be 22242")
	}

	if time.Since(time.Unix(evt.CreatedAt, 0)) > 10*time.Minute {
		return errors.New("invalid: event is too old")
	}

	challenge, err := extractTag(evt.Tags, "challenge")
	if err != nil {
		return errors.New("invalid: challenge tag missing")
	}

	relayURL, err := extractTag(evt.Tags, "relay")
	if err != nil {
		return errors.New("invalid: relay tag missing")
	}

	expectedChallenge := GetChallengeForConnection(client)
	if challenge == "" || challenge != expectedChallenge {
		return errors.New("invalid: challenge does not match or is missing")
	}

	cfg := config.GetConfig().Auth
	if !relayurl.Match(relayURL, cfg.RelayURL, relayurl.ParseMode(cfg.RelayURLMatch)) {
		return errors.New("invalid: relay URL does not match")
	}

	if !validation.CheckSignature(evt) {
		return errors.New("invalid: signature verification failed")
	}

	return nil
}

// VerifyNIP98Event verifies an HTTP Auth event per NIP-98.
//
// Callers (typically the HTTP middleware in server/api/authHelpers.go)
// supply the request context the event must commit to:
//
//   - method: the uppercase HTTP method of the request
//   - absURL: the absolute URL of the request as the server sees it,
//     reconstructed honoring X-Forwarded-Proto / X-Forwarded-Host so
//     the value matches what the client signed
//   - bodyHashHex: sha256 hex digest of the request body. Pass "" for
//     methods/requests with no body — in that case the event's payload
//     tag (if any) must also be empty/absent
//
// On success the caller may trust evt.PubKey as the request's
// authenticated identity. NIP-98 does not standardize a relay-side
// challenge/nonce; the tight created_at window is the only replay
// defense.
func VerifyNIP98Event(evt nostr.Event, method, absURL, bodyHashHex string) error {
	if evt.Kind != NIP98AuthKind {
		return errors.New("invalid: event kind must be 27235")
	}

	// Time window: NIP-98 says ~60s in either direction. We reject
	// "too old" (replay) and "too far in the future" (clock skew /
	// pre-signed events) symmetrically.
	now := time.Now()
	created := time.Unix(evt.CreatedAt, 0)
	if now.Sub(created) > nip98TimeWindow {
		return errors.New("invalid: event is too old")
	}
	if created.Sub(now) > nip98TimeWindow {
		return errors.New("invalid: event created_at is in the future")
	}

	uTag, err := extractTag(evt.Tags, "u")
	if err != nil {
		return errors.New("invalid: u tag missing")
	}
	methodTag, err := extractTag(evt.Tags, "method")
	if err != nil {
		return errors.New("invalid: method tag missing")
	}

	if methodTag != method {
		return errors.New("invalid: method tag does not match request method")
	}

	cfg := config.GetConfig().Auth
	mode := relayurl.ParseMode(cfg.RelayURLMatch)
	if !relayurl.Match(uTag, absURL, mode) {
		return errors.New("invalid: u tag does not match request URL")
	}
	// relayurl.Match drops the query string by design (it was built
	// for NIP-42 where the relay URL has no query). For NIP-98 in
	// strict mode the full URL must match, so the query is compared
	// here as a second step. In host mode the query is intentionally
	// ignored, matching the same operator preference that loosens
	// NIP-42 host matching.
	if mode == relayurl.ModeStrict {
		if !queriesMatch(uTag, absURL) {
			return errors.New("invalid: u tag does not match request URL")
		}
	}

	// Payload tag rules (NIP-98):
	//   - If the request has a body, the event MUST carry a `payload`
	//     tag whose value is the lowercase sha256 hex of that body.
	//   - If the request has no body, the `payload` tag MUST be absent
	//     (or empty). A non-empty payload tag on a body-less request
	//     is suspicious and rejected so a client can't sign a
	//     body-bearing event and replay it against a body-less route.
	payloadTag, payloadPresent := findTag(evt.Tags, "payload")
	if bodyHashHex == "" {
		if payloadPresent && payloadTag != "" {
			return errors.New("invalid: payload tag present but request has no body")
		}
	} else {
		if !payloadPresent || payloadTag == "" {
			return errors.New("invalid: payload tag missing")
		}
		if payloadTag != bodyHashHex {
			return errors.New("invalid: payload tag does not match request body hash")
		}
	}

	if !validation.CheckSignature(evt) {
		return errors.New("invalid: signature verification failed")
	}
	return nil
}

// queriesMatch reports whether two URLs share the same raw query
// string. Used only in strict-match mode for NIP-98 because
// relayurl.Match deliberately ignores queries.
func queriesMatch(a, b string) bool {
	ua, err := url.Parse(a)
	if err != nil {
		return false
	}
	ub, err := url.Parse(b)
	if err != nil {
		return false
	}
	return ua.RawQuery == ub.RawQuery
}

// findTag returns the first value for the given tag name and whether
// the tag was present. extractTag conflates "absent" and "present but
// empty"; NIP-98's payload rules need to distinguish them.
func findTag(tags [][]string, key string) (string, bool) {
	for _, tag := range tags {
		if len(tag) >= 1 && tag[0] == key {
			if len(tag) >= 2 {
				return tag[1], true
			}
			return "", true
		}
	}
	return "", false
}

// extractTag extracts a specific tag from an event's tags
func extractTag(tags [][]string, key string) (string, error) {
	for _, tag := range tags {
		if len(tag) >= 2 && tag[0] == key {
			return tag[1], nil
		}
	}
	return "", errors.New("tag not found")
}

// GetChallengeForConnection retrieves the challenge string for a given connection
func GetChallengeForConnection(client nostr.ClientInterface) string {
	authMu.Lock()
	defer authMu.Unlock()
	return challenges[client]
}

// SetChallengeForConnection sets the challenge string for a given connection
func SetChallengeForConnection(client nostr.ClientInterface, challenge string) {
	authMu.Lock()
	defer authMu.Unlock()
	log.Auth().Debug("Setting challenge for connection", "client", client)
	challenges[client] = challenge
}

// ClearChallengeForConnection removes the challenge for a connection
func ClearChallengeForConnection(client nostr.ClientInterface) {
	authMu.Lock()
	defer authMu.Unlock()
	delete(challenges, client)
}

// SetAuthenticated marks a connection as authenticated and records the
// authenticated pubkey.
func SetAuthenticated(client nostr.ClientInterface, pubkey string) {
	authMu.Lock()
	defer authMu.Unlock()
	authSessions[client] = pubkey
}

// IsAuthenticated checks if a connection is authenticated.
func IsAuthenticated(client nostr.ClientInterface) bool {
	authMu.Lock()
	defer authMu.Unlock()
	return authSessions[client] != ""
}

// GetAuthedPubkey returns the pubkey the connection authenticated as,
// or "" if the connection has not completed NIP-42 AUTH.
func GetAuthedPubkey(client nostr.ClientInterface) string {
	authMu.Lock()
	defer authMu.Unlock()
	return authSessions[client]
}
