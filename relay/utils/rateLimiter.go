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
	wsLimiter        *rate.Limiter
	eventLimiter     *rate.Limiter
	categoryLimiters map[string]*CategoryLimiter
	kindLimiters     map[int]*KindLimiter
	mu               sync.RWMutex
}

var rateLimiterInstance *RateLimiter
var once sync.Once

func SetRateLimiter(rl *RateLimiter) {
	once.Do(func() {
		rateLimiterInstance = rl
	})
}

func GetRateLimiter() *RateLimiter {
	return rateLimiterInstance
}

func NewRateLimiter(wsLimit rate.Limit, wsBurst int, eventLimit rate.Limit, eventBurst int) *RateLimiter {
	return &RateLimiter{
		wsLimiter:        rate.NewLimiter(wsLimit, wsBurst),
		eventLimiter:     rate.NewLimiter(eventLimit, eventBurst),
		categoryLimiters: make(map[string]*CategoryLimiter),
		kindLimiters:     make(map[int]*KindLimiter),
	}
}

func (rl *RateLimiter) AllowWs() bool {
	return rl.wsLimiter.Allow()
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
