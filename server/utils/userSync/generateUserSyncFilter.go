package userSync

import (
	"github.com/0ceanslim/grain/client/types"
	cfgType "github.com/0ceanslim/grain/config/types"
)

// generateUserSyncFilter creates a SubscriptionFilter based on UserSyncConfig settings.
func generateUserSyncFilter(pubKey string, syncConfig cfgType.UserSyncConfig) types.SubscriptionFilter {
	filter := types.SubscriptionFilter{
		Authors: []string{pubKey},
	}

	// Only apply Kinds if it has elements (fetch all if empty)
	if len(syncConfig.Kinds) > 0 {
		filter.Kinds = append([]int{}, syncConfig.Kinds...) // Copy slice to avoid mutation
	}

	// Only apply Limit if explicitly set (including 0)
	if syncConfig.Limit != nil {
		filter.Limit = syncConfig.Limit
	}

	return filter
}
