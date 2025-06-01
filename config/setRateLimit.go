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

type RateLimiter struct {
	wsLimiter        *rate.Limiter
	eventLimiter     *rate.Limiter
	reqLimiter       *rate.Limiter
	categoryLimiters map[string]*CategoryLimiter
	kindLimiters     map[int]*KindLimiter
	mu               sync.RWMutex
}

var rateLimiterInstance *RateLimiter
var rateOnce sync.Once

func SetRateLimit(cfg *cfgType.ServerConfig) {
	rateLimiter := NewRateLimiter(
		rate.Limit(cfg.RateLimit.WsLimit),
		cfg.RateLimit.WsBurst,
		rate.Limit(cfg.RateLimit.EventLimit),
		cfg.RateLimit.EventBurst,
		rate.Limit(cfg.RateLimit.ReqLimit),
		cfg.RateLimit.ReqBurst,
	)

	log.Config().Info("Rate limiters configured", 
    "ws_limit", cfg.RateLimit.WsLimit,
    "event_limit", cfg.RateLimit.EventLimit,
    "req_limit", cfg.RateLimit.ReqLimit)

	for _, kindLimit := range cfg.RateLimit.KindLimits {
		rateLimiter.AddKindLimit(kindLimit.Kind, rate.Limit(kindLimit.Limit), kindLimit.Burst)
		log.Config().Debug("Kind rate limiter added", "kind", kindLimit.Kind, "limit", kindLimit.Limit, "burst", kindLimit.Burst)
	}

	for category, categoryLimit := range cfg.RateLimit.CategoryLimits {
		rateLimiter.AddCategoryLimit(category, rate.Limit(categoryLimit.Limit), categoryLimit.Burst)
		log.Config().Debug("Category rate limiter added", "category", category, "limit", categoryLimit.Limit, "burst", categoryLimit.Burst)
	}

	SetRateLimiter(rateLimiter)
}

func SetRateLimiter(rl *RateLimiter) {
	rateOnce.Do(func() {
		rateLimiterInstance = rl
	})
}

func GetRateLimiter() *RateLimiter {
	return rateLimiterInstance
}

func NewRateLimiter(wsLimit rate.Limit, wsBurst int, eventLimit rate.Limit, eventBurst int, reqLimit rate.Limit, reqBurst int) *RateLimiter {
	return &RateLimiter{
		wsLimiter:        rate.NewLimiter(wsLimit, wsBurst),
		eventLimiter:     rate.NewLimiter(eventLimit, eventBurst),
		reqLimiter:       rate.NewLimiter(reqLimit, reqBurst),
		categoryLimiters: make(map[string]*CategoryLimiter),
		kindLimiters:     make(map[int]*KindLimiter),
	}
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
