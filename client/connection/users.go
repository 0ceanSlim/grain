package connection

import (
	"sync"

	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/server/utils/log"
)

// Per-user contexts on the shared core client. Everything that belongs to one
// logged-in user — the relays held open for them, their routing preferences,
// their NIP-42 authenticated connections — lives on their UserContext, so one
// user's session never changes what another user's requests do.
var (
	usersMu     sync.Mutex
	users       = make(map[string]*core.UserContext)
	usersClient *core.Client // the core client users were created on
)

// UserFor returns pubkey's UserContext on the current core client, creating it
// on first use. It returns nil when there is no core client or no pubkey.
func UserFor(pubkey string) *core.UserContext {
	cc := GetCoreClient()
	if cc == nil || pubkey == "" {
		return nil
	}
	usersMu.Lock()
	defer usersMu.Unlock()
	if usersClient != cc {
		// The core client was replaced (config reload): contexts on the old
		// one point at a closed pool.
		for _, uc := range users {
			_ = uc.Close()
		}
		users = make(map[string]*core.UserContext)
		usersClient = cc
	}
	uc := users[pubkey]
	if uc == nil {
		uc = cc.NewUserContext(pubkey)
		users[pubkey] = uc
	}
	return uc
}

// releaseAllUsers closes and forgets every user's context (core client
// shutdown).
func releaseAllUsers() {
	usersMu.Lock()
	old := users
	users = make(map[string]*core.UserContext)
	usersClient = nil
	usersMu.Unlock()
	for _, uc := range old {
		_ = uc.Close()
	}
}

// ReleaseUser closes pubkey's UserContext, releasing their relay holds and
// ending their NIP-42 sessions. Called when their last session ends.
func ReleaseUser(pubkey string) {
	usersMu.Lock()
	uc := users[pubkey]
	delete(users, pubkey)
	usersMu.Unlock()
	if uc != nil {
		_ = uc.Close()
		log.ClientConnection().Debug("Released user context", "pubkey", pubkey)
	}
}
