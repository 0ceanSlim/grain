package userSync

import (
	"fmt"
	"log/slog"

	"github.com/0ceanslim/grain/config"

	configTypes "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
)

var syncLog *slog.Logger

func init() {
   syncLog = utils.GetLogger("user-sync")
}

// UserSyncCheck determines if a user is new and triggers the initial sync if necessary.
func UserSyncCheck(evt nostr.Event, cfg *configTypes.ServerConfig) (bool, error) {
	if !cfg.UserSync.UserSync {
		syncLog.Debug("User syncing is disabled", "event_id", evt.ID)
		return false, nil
	}

	// Relay address using the port from config
	relays := []string{fmt.Sprintf("ws://localhost%s", cfg.Server.Port)}

	// Check if this is a new user
	isNewUser, err := CheckIfUserExistsOnRelay(evt.PubKey, evt.ID, relays)
	if err != nil {
		syncLog.Error("Error checking if user exists", "pubkey", evt.PubKey, "error", err)
		return false, err
	}

	if !isNewUser {
		syncLog.Debug("User is known, skipping initial sync", "pubkey", evt.PubKey)
		return false, nil
	}

	// Enforce whitelist check if ExcludeNonWhitelisted is true
	if cfg.UserSync.ExcludeNonWhitelisted {
		isWhitelisted := config.IsPubKeyWhitelisted(evt.PubKey, true)
		if !isWhitelisted {
			syncLog.Info("Non-whitelisted pubkey, skipping sync", "pubkey", evt.PubKey)
			return false, nil
		}
	}

	syncLog.Info("Starting initial sync for new user", "pubkey", evt.PubKey)

	// Trigger the sync process
	go triggerUserSync(evt.PubKey, &cfg.UserSync, cfg)

	return true, nil // New user
}