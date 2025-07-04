package data

import (
	"fmt"

	"github.com/0ceanslim/grain/client/cache"
	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

// GetUserDataForSession retrieves user metadata and mailboxes, using cache when possible
func GetUserDataForSession(publicKey string) (*nostr.Event, *core.Mailboxes, error) {
	// Try cached data first using the  cache function
	if metadata, mailboxes, found := cache.GetParsedUserData(publicKey); found {
		log.ClientData().Debug("Using cached data for session", "pubkey", publicKey)
		return metadata, mailboxes, nil
	}
	
	// Fetch fresh data using the helper function
	log.ClientData().Debug("Cache miss, fetching fresh data for session", "pubkey", publicKey)
	
	// Use the comprehensive fetch function that was moved to helpers
	if err := FetchAndCacheUserDataWithCoreClient(publicKey); err != nil {
		log.ClientData().Warn("Failed to fetch and cache user data", "pubkey", publicKey, "error", err)
		return nil, nil, err
	}
	
	// Now get the freshly cached data
	metadata, mailboxes, found := cache.GetParsedUserData(publicKey)
	if !found {
		return nil, nil, fmt.Errorf("failed to retrieve cached data after fetch")
	}
	
	return metadata, mailboxes, nil
}
