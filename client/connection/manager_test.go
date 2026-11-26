package connection

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cfgType "github.com/0ceanslim/grain/config/types"
	"golang.org/x/net/websocket"
)

// Regression: a config reload closes the core client while the previous
// InitializeCoreClient's background goroutine is still connecting. Once the
// connect finished, that goroutine started seeding and discovery on the
// package's current client, which by then was nil (a crash in CI's hot
// reload test) or a newer client. Run with -race.
func TestInitializeCoreClient_ReloadDuringBackgroundStart(t *testing.T) {
	relay := httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		var msg string
		for websocket.Message.Receive(ws, &msg) == nil {
		}
	}))
	defer relay.Close()

	cfg := &cfgType.ServerConfig{}
	cfg.Client.IndexRelays = []string{"ws" + strings.TrimPrefix(relay.URL, "http")}
	cfg.Client.RetryAttempts = 1

	for i := 0; i < 40; i++ {
		if err := InitializeCoreClient(cfg); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Duration(i%8) * 250 * time.Microsecond)
		if err := CloseCoreClient(); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(500 * time.Millisecond) // let the last background starts run
}
