package negentropy

import (
	"fmt"
	"grain/config"
	configTypes "grain/config/types"
	nostr "grain/server/types"
	"log"
)

func userSyncCheck(evt nostr.Event, cfg *configTypes.ServerConfig) {
	if !cfg.Negentropy.UserSync {
		// Syncing is disabled
		log.Printf("Negentropy syncing is disabled. Skipping sync for event %s.", evt.ID)
		return
	}

	// Relay address using the port from config
	relays := []string{fmt.Sprintf("ws://localhost%s", cfg.Server.Port)}

	// Check if this is a new user
	isNewUser, err := CheckIfUserExistsOnRelay(evt.PubKey, relays)
	if err != nil {
		log.Printf("Error checking if user exists: %v", err)
		return
	}

	if !isNewUser {
		log.Printf("User %s is known. Skipping initial sync.", evt.PubKey)
		return
	}

	// Check if non-whitelisted users are excluded
	if cfg.Negentropy.ExcludeNonWhitelisted {
		isWhitelisted, _ := config.CheckWhitelist(evt)
		if !isWhitelisted {
			log.Printf("Pubkey %s is not whitelisted. Skipping sync.", evt.PubKey)
			return
		}
	}

	log.Printf("Starting initial sync for new user %s.", evt.PubKey)

	// Trigger the sync process (to be implemented next)
	triggerUserSync() //(evt.PubKey, cfg)
}

func HandleEventSync(evt nostr.Event, cfg *configTypes.ServerConfig) {
	go userSyncCheck(evt, cfg) // Run in a goroutine for asynchronous processing
}

func triggerUserSync() {
	log.Printf("user sync triggered")
}