package config

import (
	"sync"

	cfgType "github.com/0ceanslim/grain/config/types"
	"github.com/0ceanslim/grain/server/utils/log"
)

type SizeLimiter struct {
	globalMaxSize    int
	maxContentLength int // 0 = no content-field limit
	kindSizeLimits   map[int]int
	mu               sync.RWMutex
}

func SetSizeLimit(cfg *cfgType.ServerConfig) {
	sizeLimiter := NewSizeLimiter(cfg.RateLimit.MaxEventSize)
	sizeLimiter.maxContentLength = cfg.RateLimit.MaxContentLength
	log.Config().Info("Size limiter configured",
		"global_max_size", cfg.RateLimit.MaxEventSize,
		"max_content_length", cfg.RateLimit.MaxContentLength)
	for _, kindSizeLimit := range cfg.RateLimit.KindSizeLimits {
		sizeLimiter.AddKindSizeLimit(kindSizeLimit.Kind, kindSizeLimit.MaxSize)
		log.Config().Info("Kind size limiter added", "kind", kindSizeLimit.Kind, "max_size", kindSizeLimit.MaxSize)
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
		log.Config().Debug("Event size exceeds global limit", "size", size, "limit", sl.globalMaxSize)
		return false, "Global event size limit exceeded"
	}

	if maxSize, exists := sl.kindSizeLimits[kind]; exists {
		if size > maxSize {
			log.Config().Debug("Event size exceeds kind limit", "kind", kind, "size", size, "limit", maxSize)
			return false, "Event size limit exceeded for kind"
		}
	}

	return true, ""
}

// AllowContentLength reports whether an event whose `content` field has
// contentLen characters is within the configured max. A limit of 0 means
// unlimited. Character count (not bytes) so it matches the NIP-11
// limitation.max_content_length value the relay advertises.
func (sl *SizeLimiter) AllowContentLength(contentLen int) (bool, string) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	if sl.maxContentLength > 0 && contentLen > sl.maxContentLength {
		log.Config().Debug("Event content exceeds max content length",
			"content_length", contentLen, "limit", sl.maxContentLength)
		return false, "Content length limit exceeded"
	}
	return true, ""
}
