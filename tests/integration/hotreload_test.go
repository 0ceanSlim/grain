package integration

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/0ceanslim/grain/tests"
)

// Tests run against grain-hotreload (port 8189). The config file is
// bind-mounted read-write at ../docker/configs/hotreload.yml (relative to
// this test package), so the test process can rewrite it on the host and the
// container's fsnotify watcher will pick up the change.

const hotReloadConfigPath = "../docker/configs/hotreload.yml"

// TestHotReload_ConfigChangeAppliesLive rewrites the bind-mounted config
// mid-run, waits past the fsnotify debounce, and verifies the relay
// picked up the change.
func TestHotReload_ConfigChangeAppliesLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping hot reload test in -short mode")
	}

	original, err := os.ReadFile(hotReloadConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	// Always restore the original config so we don't poison other runs.
	t.Cleanup(func() {
		_ = os.WriteFile(hotReloadConfigPath, original, 0644)
	})

	// Sanity check: permissive config accepts events.
	kp := tests.NewTestKeypair()
	client := tests.NewTestClientAt(t, tests.HotReloadRelayURL)
	evt := kp.SignEvent(1, "pre-reload", nil)
	client.SendEvent(evt)
	ok, reason := client.ExpectOK(evt.ID, 5*time.Second)
	if !ok {
		client.Close()
		t.Fatalf("expected permissive config to accept event, got %q", reason)
	}
	client.Close()

	// Rewrite the config to shrink max_event_size well below the payload
	// we'll send next. This is a minimal, targeted edit to the existing YAML.
	modified := strings.Replace(
		string(original),
		"max_event_size: 524288",
		"max_event_size: 32",
		1,
	)
	if modified == string(original) {
		t.Fatal("expected to rewrite max_event_size, but substitution failed")
	}
	if err := os.WriteFile(hotReloadConfigPath, []byte(modified), 0644); err != nil {
		t.Fatalf("write modified config: %v", err)
	}

	// Wait past the 1s fsnotify debounce plus restart time.
	time.Sleep(5 * time.Second)

	// Reconnect (the restart will have closed previous sockets) and verify
	// the new limit is in effect.
	tests.WaitForRelayReadyAt(t, tests.HotReloadHTTPURL, 30)
	client2 := tests.NewTestClientAt(t, tests.HotReloadRelayURL)
	defer client2.Close()

	evt2 := kp.SignEvent(1, "this content is definitely more than 32 bytes long and should be rejected", nil)
	client2.SendEvent(evt2)
	ok2, reason2 := client2.ExpectOK(evt2.ID, 5*time.Second)
	if ok2 {
		t.Fatal("expected event to be rejected after hot-reload tightened max_event_size")
	}
	if !strings.Contains(reason2, "Global event size limit exceeded") {
		t.Fatalf("expected size limit reject after reload, got %q", reason2)
	}
}
