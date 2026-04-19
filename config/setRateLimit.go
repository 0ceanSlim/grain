package config

import (
	"fmt"
	"sync"

	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils/log"

	"golang.org/x/time/rate"
)

type KindLimiter struct {
	Limiter *rate.Limiter
	Limit   rate.Limit
	Burst   int
}

type CategoryLimiter struct {
	Limiter *rate.Limiter
	Limit   rate.Limit
	Burst   int
}

// RateLimiter holds per-client rate limiters. Each connected client gets its
// own instance so limits are enforced independently.
type RateLimiter struct {
	wsLimiter        *rate.Limiter
	eventLimiter     *rate.Limiter
	reqLimiter       *rate.Limiter
	categoryLimiters map[string]*CategoryLimiter
	kindLimiters     map[int]*KindLimiter
	mu               sync.RWMutex
}

// rateLimitCfg stores the parsed config so new per-client limiters can be
// created on each connection without re-reading the YAML.
var (
	rateLimitCfg   *cfgType.RateLimitConfig
	rateLimitCfgMu sync.RWMutex
)

// SetRateLimit stores the rate limit configuration for later per-client use.
func SetRateLimit(cfg *cfgType.ServerConfig) {
	rateLimitCfgMu.Lock()
	rateLimitCfg = &cfg.RateLimit
	rateLimitCfgMu.Unlock()

	log.Config().Info("Rate limiters configured (per-client)",
		"ws_limit", cfg.RateLimit.WsLimit,
		"event_limit", cfg.RateLimit.EventLimit,
		"req_limit", cfg.RateLimit.ReqLimit)

	for _, kindLimit := range cfg.RateLimit.KindLimits {
		log.Config().Debug("Kind rate limiter configured", "kind", kindLimit.Kind, "limit", kindLimit.Limit, "burst", kindLimit.Burst)
	}

	for category, categoryLimit := range cfg.RateLimit.CategoryLimits {
		log.Config().Debug("Category rate limiter configured", "category", category, "limit", categoryLimit.Limit, "burst", categoryLimit.Burst)
	}
}

// NewClientRateLimiter creates a fresh RateLimiter from the stored config.
// Called once per new client connection.
func NewClientRateLimiter() *RateLimiter {
	rateLimitCfgMu.RLock()
	cfg := rateLimitCfg
	rateLimitCfgMu.RUnlock()

	if cfg == nil {
		return nil
	}

	rl := &RateLimiter{
		wsLimiter:        rate.NewLimiter(rate.Limit(cfg.WsLimit), cfg.WsBurst),
		eventLimiter:     rate.NewLimiter(rate.Limit(cfg.EventLimit), cfg.EventBurst),
		reqLimiter:       rate.NewLimiter(rate.Limit(cfg.ReqLimit), cfg.ReqBurst),
		categoryLimiters: make(map[string]*CategoryLimiter),
		kindLimiters:     make(map[int]*KindLimiter),
	}

	for _, kindLimit := range cfg.KindLimits {
		rl.kindLimiters[kindLimit.Kind] = &KindLimiter{
			Limiter: rate.NewLimiter(rate.Limit(kindLimit.Limit), kindLimit.Burst),
			Limit:   rate.Limit(kindLimit.Limit),
			Burst:   kindLimit.Burst,
		}
	}

	for category, categoryLimit := range cfg.CategoryLimits {
		rl.categoryLimiters[category] = &CategoryLimiter{
			Limiter: rate.NewLimiter(rate.Limit(categoryLimit.Limit), categoryLimit.Burst),
			Limit:   rate.Limit(categoryLimit.Limit),
			Burst:   categoryLimit.Burst,
		}
	}

	return rl
}

func (rl *RateLimiter) AllowWs() (bool, string) {
	if !rl.wsLimiter.Allow() {
		log.Config().Debug("WebSocket rate limit exceeded")
		return false, "WebSocket message rate limit exceeded"
	}
	return true, ""
}

func (rl *RateLimiter) AllowEvent(kind int, category string) (bool, string) {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if !rl.eventLimiter.Allow() {
		log.Config().Warn("Global event rate limit exceeded")
		return false, "Global event rate limit exceeded"
	}

	if kindLimiter, exists := rl.kindLimiters[kind]; exists {
		if !kindLimiter.Limiter.Allow() {
			log.Config().Debug("Rate limit exceeded for kind", "kind", kind)
			return false, fmt.Sprintf("Rate limit exceeded for kind: %d", kind)
		}
	}

	if categoryLimiter, exists := rl.categoryLimiters[category]; exists {
		if !categoryLimiter.Limiter.Allow() {
			log.Config().Debug("Rate limit exceeded for category", "category", category)
			return false, fmt.Sprintf("Rate limit exceeded for category: %s", category)
		}
	}

	return true, ""
}

func (rl *RateLimiter) AllowReq() (bool, string) {
	if !rl.reqLimiter.Allow() {
		log.Config().Debug("REQ rate limit exceeded")
		return false, "REQ rate limit exceeded"
	}
	return true, ""
}

func (rl *RateLimiter) AddCategoryLimit(category string, limit rate.Limit, burst int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.categoryLimiters[category] = &CategoryLimiter{
		Limiter: rate.NewLimiter(limit, burst),
		Limit:   limit,
		Burst:   burst,
	}
}

func (rl *RateLimiter) AddKindLimit(kind int, limit rate.Limit, burst int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.kindLimiters[kind] = &KindLimiter{
		Limiter: rate.NewLimiter(limit, burst),
		Limit:   limit,
		Burst:   burst,
	}
}
