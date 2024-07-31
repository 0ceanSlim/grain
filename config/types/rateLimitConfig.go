package config

type RateLimitConfig struct {
	WsLimit        float64                    `yaml:"ws_limit"`
	WsBurst        int                        `yaml:"ws_burst"`
	EventLimit     float64                    `yaml:"event_limit"`
	EventBurst     int                        `yaml:"event_burst"`
	ReqLimit       float64                    `yaml:"req_limit"`
	ReqBurst       int                        `yaml:"req_burst"`
	MaxEventSize   int                        `yaml:"max_event_size"`
	KindSizeLimits []KindSizeLimitConfig      `yaml:"kind_size_limits"`
	CategoryLimits map[string]KindLimitConfig `yaml:"category_limits"`
	KindLimits     []KindLimitConfig          `yaml:"kind_limits"`
}