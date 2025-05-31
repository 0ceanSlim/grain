package userSync

import (
	"fmt"
	"log/slog"

	"github.com/0ceanslim/grain/config"

	configTypes "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils"
)

// Set the logging component for general utility functions
func syncLog() *slog.Logger {
	return utils.GetLogger("user-sync")
}

// UserSyncCheckCached uses cached whitelist for sync decisions
func UserSyncCheckCached(evt nostr.Event, cfg *configTypes.ServerConfig) (bool, error) {
	if !cfg.UserSync.UserSync {
		syncLog().Debug("User syncing is disabled", "event_id", evt.ID)
		return false, nil
	}

	// Relay address using the port from config
	relays := []string{fmt.Sprintf("ws://localhost%s", cfg.Server.Port)}

	// Check if this is a new user
	isNewUser, err := CheckIfUserExistsOnRelay(evt.PubKey, evt.ID, relays)
	if err != nil {
		syncLog().Error("Error checking if user exists", "pubkey", evt.PubKey, "error", err)
		return false, err
	}

	if !isNewUser {
		syncLog().Debug("User is known, skipping initial sync", "pubkey", evt.PubKey)
		return false, nil
	}

	// Enforce whitelist check using cache if ExcludeNonWhitelisted is true
	// Use cache regardless of enabled state since this is a sync operation
	if cfg.UserSync.ExcludeNonWhitelisted {
		pubkeyCache := config.GetPubkeyCache()
		isWhitelisted := pubkeyCache.IsWhitelisted(evt.PubKey)
		if !isWhitelisted {
			syncLog().Info("Non-whitelisted pubkey, skipping sync", 
				"pubkey", evt.PubKey,
				"exclude_non_whitelisted", cfg.UserSync.ExcludeNonWhitelisted)
			return false, nil
		}
		syncLog().Debug("Pubkey is whitelisted, proceeding with sync", "pubkey", evt.PubKey)
	}

	syncLog().Info("Starting initial sync for new user", "pubkey", evt.PubKey)

	// Trigger the sync process
	go triggerUserSync(evt.PubKey, &cfg.UserSync, cfg)

	return true, nil // New user
}