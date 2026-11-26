package client

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0ceanslim/grain/client/session"
	"github.com/0ceanslim/grain/config"
	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils"
)

// The admin page renders only for a signed (verified) owner session, and
// even then doesn't carry the blocked IPs: the page loads them through a
// signed NIP-86 call (#110).
func TestAdminPage_DoesNotRenderBlockedIPs(t *testing.T) {
	SetEmbeddedWWW(os.DirFS(".."))

	owner := strings.Repeat("ab", 32)
	utils.OverrideRelayOwnerInMemory(owner)
	t.Cleanup(func() { utils.OverrideRelayOwnerInMemory("") })

	cfg := &cfgType.ServerConfig{}
	cfg.Blacklist.PermanentBlockedIPs = []string{"203.0.113.77", "198.51.100.0/24"}
	config.SetConfigForTesting(cfg)
	t.Cleanup(func() { config.SetConfigForTesting(nil) })
	wlFile := filepath.Join(t.TempDir(), "whitelist.yml")
	if err := os.WriteFile(wlFile, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config.ResetWhitelistConfig()
	if _, err := config.LoadWhitelistConfig(wlFile); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(config.ResetWhitelistConfig)

	session.SessionMgr = session.NewSessionManager()
	t.Cleanup(func() { session.SessionMgr = nil })
	login := func(verified bool) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		if _, err := session.SessionMgr.CreateSession(rec, httptest.NewRequest("POST", "/", nil),
			session.SessionInitRequest{PublicKey: owner, RequestedMode: session.WriteMode, Verified: verified}, nil); err != nil {
			t.Fatal(err)
		}
		return rec
	}

	// An unsigned session with the owner's pubkey doesn't get the page.
	req := httptest.NewRequest("GET", "/admin", nil)
	req.AddCookie(login(false).Result().Cookies()[0])
	rec := httptest.NewRecorder()
	HandleAdmin(rec, req)
	if rec.Code != 303 {
		t.Fatalf("unsigned owner session: HTTP %d, want 303", rec.Code)
	}

	req = httptest.NewRequest("GET", "/admin", nil)
	req.AddCookie(login(true).Result().Cookies()[0])
	rec = httptest.NewRecorder()
	HandleAdmin(rec, req)

	page := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("admin page: HTTP %d: %.300s", rec.Code, page)
	}
	for _, ip := range cfg.Blacklist.PermanentBlockedIPs {
		if strings.Contains(page, ip) {
			t.Errorf("admin page renders blocked IP %s", ip)
		}
	}
	if !strings.Contains(page, "data-bl-ip-show") {
		t.Error("blacklist section has no way to load the blocked IPs")
	}
}
