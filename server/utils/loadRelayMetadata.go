package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/0ceanslim/grain/server/utils/log"
)

// AuthRequiredProvider is set by higher-level packages to report the current
// auth requirement without introducing an import cycle into config.
var AuthRequiredProvider func() bool

type RelayMetadata struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Banner         string `json:"banner"`
	Icon           string `json:"icon"`
	Pubkey         string `json:"pubkey"`
	Contact        string `json:"contact"`
	SupportedNIPs  []int  `json:"supported_nips"`
	Software       string `json:"software"`
	Version        string `json:"version"`
	PrivacyPolicy  string `json:"privacy_policy"`
	TermsOfService string `json:"terms_of_service"`
	Limitation     struct {
		MaxMessageLength    int    `json:"max_message_length"`
		MaxContentLength    int    `json:"max_content_length"`
		MaxSubscriptions    int    `json:"max_subscriptions"`
		MaxLimit            int    `json:"max_limit"`
		AuthRequired        bool   `json:"auth_required"`
		PaymentRequired     bool   `json:"payment_required"`
		RestrictedWrites    bool   `json:"restricted_writes"`
		CreatedAtLowerLimit *int64 `json:"created_at_lower_limit"`
		CreatedAtUpperLimit *int64 `json:"created_at_upper_limit"`
	} `json:"limitation"`
	RelayCountries []string `json:"relay_countries"`
	LanguageTags   []string `json:"language_tags"`
	Tags           []string `json:"tags"`
	PostingPolicy  string   `json:"posting_policy"`
}

var relayMetadata RelayMetadata

// Version is set at startup from the main package's build-time ldflags
// (via server.SetVersionInfo -> utils.SetVersion). When non-empty, it
// overrides whatever `version` field is in relay_metadata.json so the
// NIP-11 info document always reflects the running binary.
var buildVersion string

// SetVersion records the build-time version string. Called from
// server.SetVersionInfo during startup; kept in this leaf package to
// avoid a server -> server/utils import cycle.
func SetVersion(v string) {
	buildVersion = v
}

func LoadRelayMetadataJSON() error {
	return LoadRelayMetadata("relay_metadata.json")
}

// GetRelayOwnerPubkey returns the relay owner's hex pubkey from
// relay_metadata.json. This is the pubkey allowed to authenticate
// against admin/management HTTP endpoints via NIP-98.
//
// Note: returns the raw on-disk value, which may be the all-zeros
// sentinel from the example metadata. Use IsRelayUnowned() when
// you want to know whether ownership has been claimed.
func GetRelayOwnerPubkey() string {
	return relayMetadata.Pubkey
}

// allZerosPubkey is the sentinel the example relay_metadata.json
// ships with — a 32-byte all-zeros key, which isn't a valid secp256k1
// point and can never be signed against, making it a safe "no owner"
// marker that's also valid JSON-schema-wise for the pubkey field.
const allZerosPubkey = "0000000000000000000000000000000000000000000000000000000000000000"

// IsRelayUnowned reports whether the relay has no owner claimed yet.
// True when the pubkey field is empty OR set to the all-zeros sentinel
// the example ships with. Used by /setup and /admin gates so a fresh
// deployment routes operators to /setup regardless of which form the
// example used.
func IsRelayUnowned() bool {
	return isRelayUnownedLocked()
}

// isRelayUnownedLocked is the same predicate without taking
// ownerClaimMu. SetRelayOwner already holds the mutex and would
// deadlock if it re-entered through the exported helper.
func isRelayUnownedLocked() bool {
	p := relayMetadata.Pubkey
	return p == "" || p == allZerosPubkey
}

// RelayMetadataWritePath is the on-disk path for the relay metadata
// JSON. Settable via SetRelayMetadataWritePath at startup so writes
// land in the same file the loader picked up (the loader currently
// hard-codes "relay_metadata.json" too, but funneling through a
// shared variable keeps the read and write halves in sync if we
// ever move it).
var relayMetadataWritePath = "relay_metadata.json"

// SetRelayMetadataWritePath records the resolved absolute path of
// the metadata file. Called from startup once the data dir is
// known. UpdateRelayMetadata uses this path so the watcher
// suppression key matches what fsnotify monitors.
func SetRelayMetadataWritePath(p string) { relayMetadataWritePath = p }

// ownerClaimMu serializes the first-run owner-claim path so two
// concurrent /setup POSTs can't both succeed. The mutex covers the
// (peek empty, write disk, reload in-memory) sequence; once an owner
// is set, subsequent SetRelayOwner calls return ErrOwnerAlreadySet
// under this same lock so the check + write is single-winner.
var ownerClaimMu sync.Mutex

// ErrOwnerAlreadySet is returned by SetRelayOwner when the on-disk
// metadata already names an owner. Callers (the /setup POST handler)
// translate this to a 409 so the page can show the "already claimed"
// state without conflating it with a real write failure.
var ErrOwnerAlreadySet = errors.New("relay owner is already set")

// SetRelayOwner persists the relay owner pubkey to relay_metadata.json
// and reloads the in-memory copy. Intended for one-time first-run
// provisioning from the /setup flow; runtime owner rotation would go
// through a future NIP-86 method, not this path.
//
// Accepts lowercased hex only — callers (the /setup handler) normalize
// first. Returns ErrOwnerAlreadySet without writing if the on-disk
// owner is non-empty, so the first POST through this function wins
// and the second sees a clean "already claimed" signal.
//
// Same atomic-write + watcher-suppression pipeline UpdateRelayMetadata
// uses, plus the package-level ownerClaimMu guarding the
// check+write atomicity.
func SetRelayOwner(pubkey string) error {
	ownerClaimMu.Lock()
	defer ownerClaimMu.Unlock()

	// Re-check against in-memory state under the lock so a parallel
	// caller that already wrote can't slip through here. We trust
	// the in-memory copy because LoadRelayMetadata at the end of the
	// previous successful call updates it before releasing the lock.
	if !isRelayUnownedLocked() {
		return ErrOwnerAlreadySet
	}

	raw, err := os.ReadFile(relayMetadataWritePath)
	if err != nil {
		return fmt.Errorf("read relay metadata: %w", err)
	}
	var patched map[string]any
	if err := json.Unmarshal(raw, &patched); err != nil {
		return fmt.Errorf("parse relay metadata: %w", err)
	}
	patched["pubkey"] = pubkey

	out, err := json.MarshalIndent(patched, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal relay metadata: %w", err)
	}
	if err := suppressAndWrite(relayMetadataWritePath, out, 0644); err != nil {
		return err
	}
	if err := LoadRelayMetadata(relayMetadataWritePath); err != nil {
		// Disk has the new owner; in-memory hasn't reloaded. That's
		// recoverable (next NIP-11 read or process restart picks it
		// up) but surface the error so the handler can surface it
		// too — operator should investigate before treating the
		// claim as fully complete.
		return fmt.Errorf("relay owner written but reload failed: %w", err)
	}
	return nil
}

// OverrideRelayOwnerInMemory sets relayMetadata.Pubkey without
// touching disk. Used by the env-var bootstrap path
// (GRAIN_OWNER_PUBKEY) so the override is re-applied on every
// startup and the on-disk file stays clean — operators who unset
// the env var don't silently inherit a stale baked-in owner.
//
// Accepts lowercased hex only; the env-var parser normalizes first.
// Holds ownerClaimMu so a /setup POST racing startup can't write a
// claim that gets overwritten in memory a moment later.
func OverrideRelayOwnerInMemory(pubkey string) {
	ownerClaimMu.Lock()
	defer ownerClaimMu.Unlock()
	relayMetadata.Pubkey = pubkey
}

// UpdateRelayMetadata applies non-nil patches to relay_metadata.json
// and reloads the in-memory copy so NIP-11 responses + the owner
// check see the new values immediately.
//
// JSON is read into a map[string]any rather than the typed
// RelayMetadata struct so unknown fields (custom NIP-11 extensions
// an operator may have added) round-trip cleanly. Marshaling
// through the typed struct would silently drop them.
//
// Used by NIP-86 changerelayname / changerelaydescription /
// changerelayicon — each passes exactly one non-nil patch. Pointer
// args so the dispatcher can express "don't touch this field" by
// passing nil.
//
// Atomic write + watcher suppression are handled here so callers
// don't have to remember. The function is safe for concurrent use:
// it holds the same package-internal mutex used by the
// suppression machinery on the config side.
func UpdateRelayMetadata(name, description, icon, banner *string) error {
	raw, err := os.ReadFile(relayMetadataWritePath)
	if err != nil {
		return fmt.Errorf("read relay metadata: %w", err)
	}
	var patched map[string]any
	if err := json.Unmarshal(raw, &patched); err != nil {
		return fmt.Errorf("parse relay metadata: %w", err)
	}
	if name != nil {
		patched["name"] = *name
	}
	if description != nil {
		patched["description"] = *description
	}
	if icon != nil {
		patched["icon"] = *icon
	}
	if banner != nil {
		patched["banner"] = *banner
	}

	out, err := json.MarshalIndent(patched, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal relay metadata: %w", err)
	}

	if err := suppressAndWrite(relayMetadataWritePath, out, 0644); err != nil {
		return err
	}

	// Reload the in-memory struct so the next NIP-11 response, owner
	// check, etc. reflect the change.
	if err := LoadRelayMetadata(relayMetadataWritePath); err != nil {
		log.Util().Warn("Failed to reload relay metadata after write", "error", err)
		// Don't fail the whole call — the file is written; subsequent
		// reads will pick up the new state eventually.
	}
	return nil
}

// WatcherSuppressor and FileWriter live in the config package; rather
// than importing it (cyclic risk via the metadata loader's reverse
// deps) the suppress + write step is funneled through a small
// package-level hook the startup code wires up. config.WatchConfigFile
// uses config.SuppressWatcherFor and config.atomicWriteFile; we
// duplicate the tiny atomic write helper here to keep server/utils
// independent.
type adminWriter func(path string, data []byte, perm os.FileMode) error
type watcherSuppressor func(path string)

var (
	writeImpl    adminWriter       = defaultAtomicWriteFile
	suppressImpl watcherSuppressor = func(string) {}
)

// SetAdminWriteHooks lets server startup wire the write + suppression
// implementations from the config package, avoiding an import cycle
// while still keeping a single source of truth for the watcher key.
func SetAdminWriteHooks(write adminWriter, suppress watcherSuppressor) {
	if write != nil {
		writeImpl = write
	}
	if suppress != nil {
		suppressImpl = suppress
	}
}

func suppressAndWrite(path string, data []byte, perm os.FileMode) error {
	suppressImpl(path)
	return writeImpl(path, data, perm)
}

// defaultAtomicWriteFile mirrors config.atomicWriteFile for the case
// where SetAdminWriteHooks hasn't been called (tests, anything that
// uses the metadata loader standalone). Same tmp+rename pattern.
func defaultAtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-")
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

func LoadRelayMetadata(filename string) error {
	log.Util().Info("Loading relay metadata", "file", filename)

	data, err := os.ReadFile(filename)
	if err != nil {
		log.Util().Error("Failed to read relay metadata file",
			"file", filename,
			"error", err)
		return err
	}

	err = json.Unmarshal(data, &relayMetadata)
	if err != nil {
		log.Util().Error("Failed to parse relay metadata JSON",
			"file", filename,
			"error", err)
		return err
	}

	log.Util().Info("Relay metadata loaded successfully",
		"relay_name", relayMetadata.Name,
		"version", relayMetadata.Version,
		"nips_count", len(relayMetadata.SupportedNIPs))

	// Log supported NIPs for debugging
	if len(relayMetadata.SupportedNIPs) > 0 {
		log.Util().Debug("Supported NIPs", "nips", relayMetadata.SupportedNIPs)
	}

	return nil
}

func RelayInfoHandler(w http.ResponseWriter, r *http.Request) {
	clientIP := GetClientIP(r)

	if r.Header.Get("Accept") != "application/nostr+json" {
		log.Util().Warn("Invalid Accept header for relay info request",
			"client_ip", clientIP,
			"accept", r.Header.Get("Accept"),
			"path", r.URL.Path)
		http.Error(w, "Unsupported Media Type", http.StatusUnsupportedMediaType)
		return
	}

	log.Util().Debug("Serving relay info",
		"client_ip", clientIP,
		"user_agent", r.UserAgent())

	w.Header().Set("Content-Type", "application/nostr+json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET")

	// Override the version field with the build-time version so the NIP-11
	// info document always matches the running binary, regardless of what
	// the on-disk relay_metadata.json says.
	response := relayMetadata
	if buildVersion != "" {
		response.Version = buildVersion
	}

	if AuthRequiredProvider != nil {
		response.Limitation.AuthRequired = AuthRequiredProvider()
	}

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Util().Error("Failed to encode relay metadata",
			"client_ip", clientIP,
			"error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Util().Info("Relay info served successfully",
		"client_ip", clientIP,
		"relay_name", response.Name,
		"version", response.Version)
}
