package config

type RateLimitConfig struct {
	WsLimit        float64                    `yaml:"ws_limit" json:"ws_limit"`
	WsBurst        int                        `yaml:"ws_burst" json:"ws_burst"`
	EventLimit     float64                    `yaml:"event_limit" json:"event_limit"`
	EventBurst     int                        `yaml:"event_burst" json:"event_burst"`
	ReqLimit       float64                    `yaml:"req_limit" json:"req_limit"`
	ReqBurst       int                        `yaml:"req_burst" json:"req_burst"`
	MaxEventSize   int                        `yaml:"max_event_size" json:"max_event_size"`
	KindSizeLimits []KindSizeLimitConfig      `yaml:"kind_size_limits" json:"kind_size_limits"`
	CategoryLimits map[string]KindLimitConfig `yaml:"category_limits" json:"category_limits"`
	KindLimits     []KindLimitConfig          `yaml:"kind_limits" json:"kind_limits"`
}
