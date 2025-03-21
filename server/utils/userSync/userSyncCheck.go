package userSync

import (
	"fmt"
	"log"

	"github.com/0ceanslim/grain/config"

	configTypes "github.com/0ceanslim/grain/config/types"
	nostr "github.com/0ceanslim/grain/server/types"
)

// UserSyncCheck determines if a user is new and triggers the initial sync if necessary.
func UserSyncCheck(evt nostr.Event, cfg *configTypes.ServerConfig) (bool, error) {
	if !cfg.UserSync.UserSync {
		log.Printf("Negentropy syncing is disabled. Skipping sync for event %s.", evt.ID)
		return false, nil
	}

	// Relay address using the port from config
	relays := []string{fmt.Sprintf("ws://localhost%s", cfg.Server.Port)}

	// Check if this is a new user
	isNewUser, err := CheckIfUserExistsOnRelay(evt.PubKey, evt.ID, relays)
	if err != nil {
		log.Printf("Error checking if user exists: %v", err)
		return false, err
	}

	if !isNewUser {
		log.Printf("User %s is known. Skipping initial sync.", evt.PubKey)
		return false, nil
	}

	// Enforce whitelist check if ExcludeNonWhitelisted is true
	if cfg.UserSync.ExcludeNonWhitelisted {
		isWhitelisted := config.IsPubKeyWhitelisted(evt.PubKey, true)
		if !isWhitelisted {
			log.Printf("Pubkey %s is not whitelisted. Skipping sync due to ExcludeNonWhitelisted.", evt.PubKey)
			return false, nil
		}
	}

	log.Printf("Starting initial sync for new user %s.", evt.PubKey)

	// Trigger the sync process
	go triggerUserSync(evt.PubKey, &cfg.UserSync, cfg)

	return true, nil // New user
}
