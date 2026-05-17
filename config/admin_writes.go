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
)

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
