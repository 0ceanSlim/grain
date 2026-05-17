// Shared primitives for the admin write surface (NIP-86 #51 phase 1
// and the grain_* extensions to follow).
//
// Two pieces here:
//
//   - ConfigMu: a single package-wide mutex that callers acquire
//     before any read-modify-write on a config global. The
//     pre-existing AddToPermanentBlacklist did its read-modify-write
//     unlocked, which races on concurrent admin requests. Phase 1
//     refactors that path onto this lock; new helpers acquire it
//     too. Admin writes are infrequent enough that contention is
//     not a real concern.
//
//   - atomicWriteFile: tmp + rename, the same pattern
//     config/IPBlacklist.go:writeIPSidecar uses for the sidecar.
//     Crash-safe on POSIX *and* Windows (bare os.WriteFile is not
//     crash-safe on Windows because it truncates before writing).
//     A partial write can never leave a half-rendered YAML that
//     would break a reload.
//
// Every admin write helper is expected to:
//
//   1. Validate input (no I/O before bad data is rejected).
//   2. ConfigMu.Lock() / defer ConfigMu.Unlock().
//   3. Mutate the in-memory config global.
//   4. Marshal.
//   5. SuppressWatcherFor(path) so fsnotify doesn't fire a restart.
//   6. atomicWriteFile(path, ...).
//   7. Trigger any cache refresh.
//
// Skipping step 5 results in a 3-second full server restart per
// admin action, which drops every WebSocket connection including
// the one that issued the request.

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"gopkg.in/yaml.v3"
)

// triggerRestartImpl is wired at startup so grain_reloadconfig can
// kick a restart through the existing restart-channel loop without
// importing server (which would cycle). Default no-op so tests
// running the config package in isolation don't blow up.
var triggerRestartImpl func() = func() {
	log.Config().Warn("config: TriggerRestart called before hook wired; no-op")
}

// SetTriggerRestart installs the hook that pushes a value onto the
// restart channel. server/startup.go wires the real implementation
// once that channel exists.
func SetTriggerRestart(fn func()) { triggerRestartImpl = fn }

// TriggerRestart asks the server to restart through the existing
// loop in server/startup.go. The HTTP response to whoever called
// this has already returned by the time the restart fires —
// fsnotify-suppressed admin writes mean only an explicit call
// kicks the restart, which is the whole point of staging config
// updates.
func TriggerRestart() { triggerRestartImpl() }

// ConfigMu serializes admin writes across the whole config package.
// Public so write helpers in config/Blacklist.go, config/Whitelist.go,
// config/IPBlacklist.go and server/utils/loadRelayMetadata.go can all
// share one critical section.
var ConfigMu sync.Mutex

// AtomicWriteFile writes data to a temp file in the same directory
// as `path` and then renames it into place. Atomic on POSIX and
// Windows when the target is a regular file on a writable
// filesystem.
//
// Bind-mount fallback: when `path` is bind-mounted as an individual
// file (Docker, podman, Kubernetes ConfigMap projections), the host
// kernel refuses `rename` over it with EBUSY because the mount
// holds the inode. The same situation comes up when ops run grain
// behind systemd's PrivateTmp or similar overlays. Detecting this
// reliably is hard; the pragmatic fallback is "if rename fails for
// any reason, try a truncating in-place write instead". That loses
// atomicity in those environments — a crash mid-write can leave a
// partial file — but the alternative is admin writes failing
// outright, which is worse for the dashboard UX. Production
// deployments on a normal filesystem still get the atomic path.
//
// Exported so server/utils can install it as the relay-metadata
// write hook (see startup.go) without an import cycle. Callers
// inside this package also use the exported name.
//
// On failure the temp file is removed best-effort so we don't leave
// .tmp* litter; callers shouldn't rely on cleanup since the error
// is the more important signal.
// saveServerConfig marshals the supplied ServerConfig to config.yml.
// Suppresses the watcher so admin saves don't trigger an
// automatic restart — phase 2 of NIP-86 stages config changes; the
// dashboard hits grain_reloadconfig explicitly when the operator
// wants to apply pending changes.
//
// Caller MUST hold ConfigMu. Returns the marshaled bytes path on
// success (atomic write with bind-mount fallback handled by
// AtomicWriteFile).
func saveServerConfig(cfg cfgType.ServerConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	path := ConfigPath("config.yml")
	SuppressWatcherFor(path)
	if err := AtomicWriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config.yml: %w", err)
	}
	return nil
}

// UpdateRateLimitConfig stages a new rate-limit configuration: the
// supplied blob replaces the current cfg.RateLimit and config.yml
// is rewritten. Running rate limiters keep their buckets until the
// next reload (grain_reloadconfig); reads of cfg via GetConfig()
// see the new section immediately.
func UpdateRateLimitConfig(rl cfgType.RateLimitConfig) error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()
	c := GetConfig()
	if c == nil {
		return fmt.Errorf("config not loaded")
	}
	c.RateLimit = rl
	return saveServerConfig(*c)
}

// UpdateEventPurgeConfig stages a new event-purge configuration.
// Purge timers stay on the old schedule until reload.
func UpdateEventPurgeConfig(ep cfgType.EventPurgeConfig) error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()
	c := GetConfig()
	if c == nil {
		return fmt.Errorf("config not loaded")
	}
	c.EventPurge = ep
	return saveServerConfig(*c)
}

// UpdateLoggingConfig stages logging settings. The existing slog
// handlers keep writing to the old file/level until reload —
// rebuilding the logging tree mid-run is fiddly and not worth the
// complexity given reload exists.
func UpdateLoggingConfig(lg cfgType.LogConfig) error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()
	c := GetConfig()
	if c == nil {
		return fmt.Errorf("config not loaded")
	}
	c.Logging = lg
	return saveServerConfig(*c)
}

// UpdateAuthConfig stages auth settings (required flag, relay_url,
// relay_url_match mode). The validation that consults these reads
// from GetConfig().Auth on each AUTH event, so the in-memory swap
// makes new sessions see the new policy without restart. Existing
// authenticated WS connections retain their session, which matches
// operator intent (don't kick logged-in users for a policy tweak).
func UpdateAuthConfig(au cfgType.AuthConfig) error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()
	c := GetConfig()
	if c == nil {
		return fmt.Errorf("config not loaded")
	}
	c.Auth = au
	return saveServerConfig(*c)
}

// UpdateBackupRelayConfig stages the backup-relay forwarding
// settings. The backup-relay goroutine reads these at startup; a
// reload reinitializes it.
func UpdateBackupRelayConfig(br cfgType.BackupRelayConfig) error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()
	c := GetConfig()
	if c == nil {
		return fmt.Errorf("config not loaded")
	}
	c.BackupRelay = br
	return saveServerConfig(*c)
}

// UpdateResourceLimits stages CPU/memory caps. The runtime/debug
// hooks (GOMAXPROCS, soft memory limit) are applied at startup;
// reload reapplies them.
func UpdateResourceLimits(rl cfgType.ResourceLimits) error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()
	c := GetConfig()
	if c == nil {
		return fmt.Errorf("config not loaded")
	}
	c.ResourceLimits = rl
	return saveServerConfig(*c)
}

// UpdateEventTimeConstraints stages the min/max created_at window.
// The validator reads from GetConfig() per event, so this is
// effectively live.
func UpdateEventTimeConstraints(etc cfgType.EventTimeConstraints) error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()
	c := GetConfig()
	if c == nil {
		return fmt.Errorf("config not loaded")
	}
	c.EventTimeConstraints = etc
	return saveServerConfig(*c)
}

// UpdateServerConfig stages the HTTP server block (timeouts,
// connection caps, max subscriptions). Timeouts on the running
// http.Server can't be changed in place; reload picks up new
// values.
func UpdateServerConfig(srv cfgType.ServerSettings) error {
	ConfigMu.Lock()
	defer ConfigMu.Unlock()
	c := GetConfig()
	if c == nil {
		return fmt.Errorf("config not loaded")
	}
	c.Server = srv
	return saveServerConfig(*c)
}

// AtomicWriteFile writes data to a temp file in the same directory
// as `path` and then renames it into place. Atomic on POSIX and
// Windows when the target is a regular file on a writable
// filesystem.
//
// Bind-mount fallback: when `path` is bind-mounted as an individual
// file (Docker, podman, Kubernetes ConfigMap projections), the host
// kernel refuses `rename` over it with EBUSY because the mount
// holds the inode. The pragmatic fallback is "if rename fails for
// any reason, try a truncating in-place write instead" — loses
// atomicity in those environments but admin writes still land.
//
// Exported so server/utils can install it as the relay-metadata
// write hook (see startup.go) without an import cycle.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-")
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("chmod tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		// Bind-mount fallback. Write in place so the file
		// actually gets updated; non-atomic, but correct.
		if writeErr := os.WriteFile(path, data, perm); writeErr != nil {
			return fmt.Errorf("rename tmp into place: %v; in-place fallback also failed: %v", err, writeErr)
		}
		return nil
	}
	return nil
}
