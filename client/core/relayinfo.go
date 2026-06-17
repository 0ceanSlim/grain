package core

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// RelayInfo is the subset of a relay's NIP-11 document the relay manager shows.
type RelayInfo struct {
	Name          string       `json:"name,omitempty"`
	Description   string       `json:"description,omitempty"`
	PubKey        string       `json:"pubkey,omitempty"`
	Software      string       `json:"software,omitempty"`
	Version       string       `json:"version,omitempty"`
	SupportedNIPs []int        `json:"supported_nips,omitempty"`
	Icon          string       `json:"icon,omitempty"`
	Limitation    *RelayLimits `json:"limitation,omitempty"`
}

// RelayLimits is the NIP-11 `limitation` block — the flags that matter to the UI
// (whether the relay requires AUTH or payment to use).
type RelayLimits struct {
	AuthRequired     bool `json:"auth_required,omitempty"`
	PaymentRequired  bool `json:"payment_required,omitempty"`
	RestrictedWrites bool `json:"restricted_writes,omitempty"`
	MaxMessageLength int  `json:"max_message_length,omitempty"`
}

type relayInfoEntry struct {
	info *RelayInfo
	at   time.Time
}

const relayInfoTTL = 6 * time.Hour

var relayInfoHTTP = &http.Client{Timeout: 6 * time.Second}

// FetchRelayInfo returns a relay's NIP-11 document, TTL-cached. It is a plain
// HTTP GET of the relay's root with `Accept: application/nostr+json` — not a
// pool/WebSocket connection — so the known-relays browser can show name,
// software, supported NIPs, and the auth/payment flags. Returns nil when the
// relay serves no NIP-11 or it can't be parsed (also cached, to avoid retry
// storms on a relay that doesn't advertise one).
func (c *Client) FetchRelayInfo(url string) *RelayInfo {
	if norm, ok := normalizeRelayURL(url); ok {
		url = norm
	}
	c.relayInfoMu.Lock()
	if e, ok := c.relayInfoCache[url]; ok && time.Since(e.at) < relayInfoTTL {
		c.relayInfoMu.Unlock()
		return e.info
	}
	c.relayInfoMu.Unlock()

	info := fetchNIP11(url)

	c.relayInfoMu.Lock()
	c.relayInfoCache[url] = relayInfoEntry{info: info, at: time.Now()}
	c.relayInfoMu.Unlock()
	return info
}

// fetchNIP11 does the HTTP GET for a relay's NIP-11 document, converting the
// ws(s) URL to http(s).
func fetchNIP11(wsURL string) *RelayInfo {
	httpURL := strings.Replace(wsURL, "wss://", "https://", 1)
	httpURL = strings.Replace(httpURL, "ws://", "http://", 1)

	req, err := http.NewRequest(http.MethodGet, httpURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Accept", "application/nostr+json")

	resp, err := relayInfoHTTP.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var info RelayInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil
	}
	return &info
}
