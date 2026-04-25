package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/client/core/tools"
	cfgType "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"gopkg.in/yaml.v3"
)

// CheckBlacklistCached uses cached pubkey lists and respects enabled state for validation
func CheckBlacklistCached(pubkey, eventContent string) (bool, string) {
	blacklistConfig := GetBlacklistConfig()
	if blacklistConfig == nil || !blacklistConfig.Enabled {
		return false, ""
	}

	log.Config().Debug("Checking cached blacklist for pubkey", "pubkey", pubkey)

	pubkeyCache := GetPubkeyCache()

	// Check cached permanent blacklist (respects enabled state for validation)
	if pubkeyCache.IsBlacklistedForValidation(pubkey) {
		log.Config().Warn("Pubkey found in cached blacklist", "pubkey", pubkey)
		return true, "blocked: pubkey is blacklisted"
	}

	// Check for temporary ban (this still needs real-time checking)
	if isPubKeyTemporarilyBlacklisted(pubkey) {
		log.Config().Warn("Pubkey temporarily blacklisted", "pubkey", pubkey)
		return true, "blocked: pubkey is temporarily blacklisted"
	}

	// Check for permanent ban based on content (wordlist)
	for _, word := range blacklistConfig.PermanentBanWords {
		if strings.Contains(eventContent, word) {
			err := AddToPermanentBlacklist(pubkey)
			if err != nil {
				log.Config().Error("Failed to add pubkey to permanent blacklist",
					"pubkey", pubkey,
					"word", word,
					"error", err)
				return true, fmt.Sprintf("pubkey %s is permanently banned and failed to save: %v", pubkey, err)
			}

			// Trigger immediate blacklist refresh to include this pubkey
			go GetPubkeyCache().RefreshBlacklist()

			log.Config().Warn("Pubkey permanently banned due to wordlist match",
				"pubkey", pubkey,
				"word", word)
			return true, "blocked: pubkey is permanently banned"
		}
	}

	// Check for temporary ban based on content (wordlist)
	for _, word := range blacklistConfig.TempBanWords {
		if strings.Contains(eventContent, word) {
			err := AddToTemporaryBlacklist(pubkey, *blacklistConfig)
			if err != nil {
				log.Config().Error("Failed to add pubkey to temporary blacklist",
					"pubkey", pubkey,
					"word", word,
					"error", err)
				return true, fmt.Sprintf("pubkey %s is temporarily banned and failed to save: %v", pubkey, err)
			}
			log.Config().Warn("Pubkey temporarily banned due to wordlist match",
				"pubkey", pubkey,
				"word", word)
			return true, "blocked: pubkey is temporarily banned"
		}
	}

	return false, ""
}

// Checks if a pubkey is temporarily blacklisted
func isPubKeyTemporarilyBlacklisted(pubkey string) bool {
	mu.Lock()
	defer mu.Unlock()

	entry, exists := tempBannedPubkeys[pubkey]
	if !exists {
		log.Config().Debug("Pubkey not in temporary blacklist", "pubkey", pubkey)
		return false
	}

	now := time.Now()
	if now.After(entry.unbanTime) {
		log.Config().Info("Temporary ban expired",
			"pubkey", pubkey,
			"count", entry.count,
			"unban_time", entry.unbanTime.Format(time.RFC3339))
		return false
	}

	log.Config().Warn("Pubkey currently temporarily blacklisted",
		"pubkey", pubkey,
		"count", entry.count,
		"unban_time", entry.unbanTime.Format(time.RFC3339))
	return true
}

func ClearTemporaryBans() {
	mu.Lock()
	defer mu.Unlock()
	tempBannedPubkeys = make(map[string]*tempBanEntry)
	log.Config().Debug("Cleared all temporary bans", "timestamp", time.Now().Format(time.RFC3339))
}

var (
	tempBannedPubkeys = make(map[string]*tempBanEntry)
)

type tempBanEntry struct {
	count     int
	unbanTime time.Time
}

// Adds a pubkey to the temporary blacklist
func AddToTemporaryBlacklist(pubkey string, blacklistConfig cfgType.BlacklistConfig) error {
	mu.Lock()
	defer mu.Unlock()

	entry, exists := tempBannedPubkeys[pubkey]
	if !exists {
		log.Config().Info("Creating new temporary ban entry", "pubkey", pubkey)
		entry = &tempBanEntry{
			count:     0,
			unbanTime: time.Now(),
		}
		tempBannedPubkeys[pubkey] = entry
	} else {
		log.Config().Info("Updating existing temporary ban entry",
			"pubkey", pubkey,
			"current_count", entry.count)

		if time.Now().After(entry.unbanTime) {
			log.Config().Info("Previous ban expired, keeping count",
				"pubkey", pubkey,
				"count", entry.count)
		}
	}

	entry.count++
	entry.unbanTime = time.Now().Add(time.Duration(blacklistConfig.TempBanDuration) * time.Second)

	log.Config().Debug("Updated temporary ban",
		"pubkey", pubkey,
		"count", entry.count,
		"max_temp_bans", blacklistConfig.MaxTempBans,
		"unban_time", entry.unbanTime.Format(time.RFC3339))

	if entry.count > blacklistConfig.MaxTempBans {
		log.Config().Warn("Max temporary bans exceeded, moving to permanent blacklist",
			"pubkey", pubkey,
			"count", entry.count)

		delete(tempBannedPubkeys, pubkey)

		mu.Unlock()
		err := AddToPermanentBlacklist(pubkey)
		mu.Lock()

		if err != nil {
			log.Config().Error("Failed to move pubkey to permanent blacklist",
				"pubkey", pubkey,
				"error", err)
			return err
		}
		log.Config().Info("Successfully moved pubkey to permanent blacklist", "pubkey", pubkey)

		// Trigger an async pubkey-cache refresh so subsequent events from
		// this pubkey are actually rejected by CheckBlacklistCached — the
		// wordlist path does this; the escalation path was missing it,
		// leaving the pubkey newly in the file/slice but not yet in the
		// cache the validator actually reads.
		go GetPubkeyCache().RefreshBlacklist()
	}

	return nil
}

// GetTemporaryBlacklist fetches all currently active temporary bans
func GetTemporaryBlacklist() []map[string]interface{} {
	mu.Lock()
	defer mu.Unlock()

	var tempBans []map[string]interface{}

	now := time.Now()
	expired := 0

	for pubkey, entry := range tempBannedPubkeys {
		if now.Before(entry.unbanTime) {
			tempBans = append(tempBans, map[string]interface{}{
				"pubkey":     pubkey,
				"expires_at": entry.unbanTime.Unix(),
			})
		} else {
			log.Config().Info("Removing expired temp ban", "pubkey", pubkey)
			delete(tempBannedPubkeys, pubkey)
		}
	}

	if expired > 0 {
		log.Config().Debug("Cleaned up expired temporary bans", "count", expired)
	}

	return tempBans
}

func isPubKeyPermanentlyBlacklisted(pubKey string, blacklistConfig *cfgType.BlacklistConfig) bool {
	if blacklistConfig == nil || !blacklistConfig.Enabled {
		return false
	}

	for _, blacklistedKey := range blacklistConfig.PermanentBlacklistPubkeys {
		if pubKey == blacklistedKey {
			return true
		}
	}

	for _, npub := range blacklistConfig.PermanentBlacklistNpubs {
		decodedPubKey, err := tools.DecodeNpub(npub)
		if err != nil {
			log.Config().Error("Error decoding npub", "npub", npub, "error", err)
			continue
		}
		if pubKey == decodedPubKey {
			return true
		}
	}

	return false
}

func AddToPermanentBlacklist(pubkey string) error {
	blacklistConfig := GetBlacklistConfig()
	if blacklistConfig == nil {
		return fmt.Errorf("blacklist configuration is not loaded")
	}

	if isPubKeyPermanentlyBlacklisted(pubkey, blacklistConfig) {
		log.Config().Debug("Pubkey already in permanent blacklist", "pubkey", pubkey)
		return fmt.Errorf("pubkey %s is already in the permanent blacklist", pubkey)
	}

	blacklistConfig.PermanentBlacklistPubkeys = append(blacklistConfig.PermanentBlacklistPubkeys, pubkey)

	log.Config().Info("Added pubkey to permanent blacklist", "pubkey", pubkey)

	err := saveBlacklistConfig(*blacklistConfig)
	if err != nil {
		log.Config().Error("Failed to save blacklist configuration", "error", err)
		return err
	}

	log.Config().Debug("Saved blacklist configuration to file")
	return nil
}

func saveBlacklistConfig(blacklistConfig cfgType.BlacklistConfig) error {
	data, err := yaml.Marshal(blacklistConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal blacklist config: %v", err)
	}

	err = os.WriteFile("blacklist.yml", data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config to file: %v", err)
	}

	return nil
}

// muteListKinds are the NIP-51 list kinds consulted as blacklist sources:
//   - 10000: standard Mute list (replaceable)
//   - 30000: Categorized people list (addressable). By convention, `d:"mute"`
//     identifies a mute list; other `d` values (e.g. "family") are ignored.
var muteListKinds = []int{10000, 30000}

// muteListFetchTimeout bounds how long we wait per-author for mute list
// events to arrive after sending the REQ.
const muteListFetchTimeout = 8 * time.Second

// FetchGroupedMuteListPubkeys returns public `p`-tag pubkeys from each configured
// author's NIP-51 mute list events, grouped by author pubkey.
//
// For each author, the fetch path is:
//  1. Look up the author's NIP-65 mailbox list (kind:10002) via the client
//     library's connected index relays.
//  2. Target their outbox relays (write + both). If none are published or
//     reachable, fall back to the relay's configured default client relays.
//  3. Subscribe for kinds 10000 and 30000 from that author.
//  4. Keep only the latest event per (kind, d-tag) — replaceable/addressable
//     semantics — and for kind 30000 require `d:"mute"` (filtered here
//     because the client library's Filter type does not currently serialize
//     NIP-01 `#<tag>` tag filters in the REQ wire format).
//  5. Extract public `p`-tag pubkeys from the winning events.
//
// Encrypted `.content` entries (NIP-44 primarily, NIP-04 fallback per NIP-51)
// are not decrypted by the relay — only public tag entries are applied.
func FetchGroupedMuteListPubkeys(authors []string) (map[string][]string, error) {
	result := make(map[string][]string)
	if len(authors) == 0 {
		return result, nil
	}

	client := connection.GetCoreClient()
	if client == nil {
		log.Config().Warn("Core client not initialized — mutelist fetch skipped",
			"author_count", len(authors))
		return result, nil
	}

	withPubkeys := 0
	for _, author := range authors {
		// Always record the author in the result, even when zero public
		// pubkeys were extracted — the dashboard otherwise loses sight of
		// configured authors whose mute lists are encrypted or unreachable.
		// Callers that count contributed pubkeys should iterate the values,
		// not the keys.
		pubkeys := fetchAuthorMuteListPubkeys(client, author)
		result[author] = pubkeys
		if len(pubkeys) > 0 {
			withPubkeys++
		}
	}

	log.Config().Debug("Grouped mutelist fetch complete",
		"authors_configured", len(authors),
		"authors_with_pubkeys", withPubkeys)
	return result, nil
}

// fetchAuthorMuteListPubkeys runs the per-author outbox lookup + mute list
// subscription described in FetchGroupedMuteListPubkeys.
func fetchAuthorMuteListPubkeys(client *core.Client, author string) []string {
	targets := resolveMuteListRelays(client, author)
	if len(targets) == 0 {
		log.Config().Warn("No relays available for mutelist author",
			"author", author)
		return nil
	}

	// Connect to any targets the pool doesn't already hold. Errors here just
	// mean some targets were unreachable; Subscribe will skip those and use
	// the rest.
	_ = client.ConnectToRelays(targets)

	filter := nostr.Filter{
		Authors: []string{author},
		Kinds:   muteListKinds,
	}
	sub, err := client.Subscribe([]nostr.Filter{filter}, targets)
	if err != nil {
		log.Config().Error("Failed to subscribe for mute list",
			"author", author, "error", err)
		return nil
	}
	defer sub.Close()

	events := collectMuteListEvents(sub, targets, muteListFetchTimeout)
	winners := latestMuteListEventsPerKindD(events)
	return extractMuteListPubkeys(winners, author)
}

// resolveMuteListRelays returns the relay set to query for the given author's
// mute list events: the union of their NIP-65 outbox relays (write + both)
// and the relay's configured default client relays, deduplicated.
//
// We query both rather than preferring outbox-only because authors often
// publish kind:10000 to popular relays (relay.damus.io, nos.lol, etc.)
// that aren't in their declared outbox list. Outbox-only coverage misses
// these and yields zero public pubkeys for the author. Querying defaults
// alongside outbox doubles our chances of finding the event without
// meaningfully increasing per-fetch cost — the default set is already
// connected from app startup.
func resolveMuteListRelays(client *core.Client, author string) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(urls []string) {
		for _, u := range urls {
			if u == "" || seen[u] {
				continue
			}
			seen[u] = true
			out = append(out, u)
		}
	}

	mailboxes, err := client.GetUserRelays(author)
	if err == nil && mailboxes != nil {
		add(mailboxes.Write)
		add(mailboxes.Both)
	} else if err != nil {
		log.Config().Debug("NIP-65 lookup failed for mutelist author",
			"author", author, "error", err)
	}
	outboxCount := len(out)

	add(connection.GetIndexRelays())

	log.Config().Debug("Resolved mutelist relays",
		"author", author,
		"outbox_relays", outboxCount,
		"index_relays_added", len(out)-outboxCount,
		"total", len(out))
	return out
}

// collectMuteListEvents drains the subscription's Events channel, returning
// when every target relay has sent EOSE or the timeout fires.
func collectMuteListEvents(sub *core.Subscription, relays []string, timeout time.Duration) []*nostr.Event {
	var events []*nostr.Event
	eose := make(map[string]bool)
	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-sub.Events:
			if !ok {
				return events
			}
			if ev != nil {
				events = append(events, ev)
			}
		case relayURL, ok := <-sub.EOSE:
			if !ok {
				return events
			}
			eose[relayURL] = true
			if len(eose) >= len(relays) {
				return events
			}
		case <-deadline:
			log.Config().Debug("Mute list subscription timed out",
				"events_received", len(events),
				"eose_received", len(eose),
				"relays", len(relays))
			return events
		}
	}
}

// latestMuteListEventsPerKindD implements NIP-01 replaceable/addressable
// semantics: for each (kind, d-tag) tuple, only the highest `created_at`
// wins. kind:30000 events without `d:"mute"` are discarded here — only the
// "mute" category counts as a blacklist source.
func latestMuteListEventsPerKindD(events []*nostr.Event) []*nostr.Event {
	type key struct {
		kind int
		d    string
	}
	latest := make(map[key]*nostr.Event)
	for _, ev := range events {
		if ev == nil {
			continue
		}
		d := firstTagValue(ev.Tags, "d")
		if ev.Kind == 30000 && d != "mute" {
			continue
		}
		k := key{kind: ev.Kind, d: d}
		if cur, ok := latest[k]; !ok || ev.CreatedAt > cur.CreatedAt {
			latest[k] = ev
		}
	}
	out := make([]*nostr.Event, 0, len(latest))
	for _, ev := range latest {
		out = append(out, ev)
	}
	return out
}

// extractMuteListPubkeys returns deduplicated public `p`-tag pubkeys from the
// winning mute list events. Entries encrypted inside `.content` are not
// decrypted; a debug log flags their presence so operators can see that some
// mutes exist but are unreachable.
func extractMuteListPubkeys(events []*nostr.Event, author string) []string {
	seen := make(map[string]bool)
	var pubkeys []string
	for _, ev := range events {
		if ev == nil {
			continue
		}
		if ev.Content != "" {
			log.Config().Debug("Mute list event has encrypted content the relay cannot decrypt",
				"author", author, "kind", ev.Kind, "event_id", ev.ID)
		}
		for _, tag := range ev.Tags {
			if len(tag) < 2 || tag[0] != "p" {
				continue
			}
			pk := tag[1]
			if pk == "" || seen[pk] {
				continue
			}
			seen[pk] = true
			pubkeys = append(pubkeys, pk)
		}
	}
	return pubkeys
}

// firstTagValue returns the value of the first tag with the given name, or
// "" if none. Only tag[1] is inspected (the NIP-01 value slot).
func firstTagValue(tags [][]string, name string) string {
	for _, tag := range tags {
		if len(tag) >= 2 && tag[0] == name {
			return tag[1]
		}
	}
	return ""
}
