package config

type AuthConfig struct {
	Required bool   `yaml:"required" json:"required"`
	RelayURL string `yaml:"relay_url" json:"relay_url"`
	// RelayURLMatch controls how the AUTH event's `relay` tag is
	// compared against RelayURL. "strict" (default) keeps the path
	// significant after canonicalization; "host" drops the path so any
	// AUTH addressed at the right host is accepted. Empty == "strict".
	// See server/utils/relayurl for the exact rules.
	RelayURLMatch string `yaml:"relay_url_match" json:"relay_url_match"`
}
