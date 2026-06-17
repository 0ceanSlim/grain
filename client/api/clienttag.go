package api

import (
	"github.com/0ceanslim/grain/client/core"
	"github.com/0ceanslim/grain/config"
)

// resolveClientTag decides whether grain stamps its `client` tag on an event it
// builds, and under what name. The configured default (#80/#99) is overridden
// per-build by the user's settings slider — `override` is nil when the request
// carries no preference, in which case the default applies. Foreign `client`
// tags are always stripped regardless (see core.ApplyClientTag).
func resolveClientTag(override *bool) (enabled bool, name string) {
	enabled, name = true, core.DefaultClientTagName
	if cfg := config.GetConfig(); cfg != nil {
		enabled = cfg.Client.ClientTag.Enabled
		if cfg.Client.ClientTag.Name != "" {
			name = cfg.Client.ClientTag.Name
		}
	}
	if override != nil {
		enabled = *override
	}
	return enabled, name
}
