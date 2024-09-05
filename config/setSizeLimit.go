package config

import (
	"sync"

	config "grain/config/types"
)

type SizeLimiter struct {
	globalMaxSize  int
	kindSizeLimits map[int]int
	mu             sync.RWMutex
}

func SetSizeLimit(cfg *config.ServerConfig) {
	sizeLimiter := NewSizeLimiter(cfg.RateLimit.MaxEventSize)
	for _, kindSizeLimit := range cfg.RateLimit.KindSizeLimits {
		sizeLimiter.AddKindSizeLimit(kindSizeLimit.Kind, kindSizeLimit.MaxSize)
	}

	SizeLimit(sizeLimiter)
}

func NewSizeLimiter(globalMaxSize int) *SizeLimiter {
	return &SizeLimiter{
		globalMaxSize:  globalMaxSize,
		kindSizeLimits: make(map[int]int),
	}
}

var sizeLimiterInstance *SizeLimiter
var sizeOnce sync.Once

func GetSizeLimiter() *SizeLimiter {
	return sizeLimiterInstance
}

func SizeLimit(sl *SizeLimiter) {
	sizeOnce.Do(func() {
		sizeLimiterInstance = sl
	})
}

func (sl *SizeLimiter) SetGlobalMaxSize(maxSize int) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.globalMaxSize = maxSize
}

func (sl *SizeLimiter) AddKindSizeLimit(kind int, maxSize int) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.kindSizeLimits[kind] = maxSize
}

func (sl *SizeLimiter) AllowSize(kind int, size int) (bool, string) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	if size > sl.globalMaxSize {
		return false, "Global event size limit exceeded"
	}

	if maxSize, exists := sl.kindSizeLimits[kind]; exists {
		if size > maxSize {
			return false, "Event size limit exceeded for kind"
		}
	}

	return true, ""
}
