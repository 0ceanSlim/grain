package utils

import (
	"sync"

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
	eventLimiter     *rate.Limiter
	wsLimiter        *rate.Limiter
	kindLimiters     map[int]*KindLimiter
	categoryLimiters map[string]*CategoryLimiter
	mu               sync.RWMutex
}

var rateLimiterInstance *RateLimiter
var once sync.Once

func NewRateLimiter(eventLimit rate.Limit, eventBurst int, wsLimit rate.Limit, wsBurst int) *RateLimiter {
	return &RateLimiter{
		eventLimiter:     rate.NewLimiter(eventLimit, eventBurst),
		wsLimiter:        rate.NewLimiter(wsLimit, wsBurst),
		kindLimiters:     make(map[int]*KindLimiter),
		categoryLimiters: make(map[string]*CategoryLimiter),
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

func (rl *RateLimiter) AddCategoryLimit(category string, limit rate.Limit, burst int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.categoryLimiters[category] = &CategoryLimiter{
		Limiter: rate.NewLimiter(limit, burst),
		Limit:   limit,
		Burst:   burst,
	}
}

func (rl *RateLimiter) AllowEvent(kind int, category string) bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	if !rl.eventLimiter.Allow() {
		return false
	}

	if kindLimiter, exists := rl.kindLimiters[kind]; exists {
		if !kindLimiter.Limiter.Allow() {
			return false
		}
	}

	if categoryLimiter, exists := rl.categoryLimiters[category]; exists {
		if !categoryLimiter.Limiter.Allow() {
			return false
		}
	}

	return true
}

func (rl *RateLimiter) AllowWs() bool {
	return rl.wsLimiter.Allow()
}

func SetRateLimiter(rl *RateLimiter) {
	once.Do(func() {
		rateLimiterInstance = rl
	})
}

func GetRateLimiter() *RateLimiter {
	return rateLimiterInstance
}
