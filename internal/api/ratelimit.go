package api

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// How many allow() calls pass between sweeps of idle keys. Without a sweep the
// map grows for the lifetime of the process, one entry per source address ever
// seen, which a caller rotating addresses turns into a memory leak.
const sweepEvery = 256

type limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	limit  int
	window time.Duration
	calls  int
}

// RateLimit permits limit requests per client per sliding window, answering 429
// past that.
//
// Keyed on the socket peer rather than gin's ClientIP(): ClientIP() honours
// X-Forwarded-For from any source unless trusted proxies are configured, so that
// key would let one caller rotate a header and spend everyone's budget. Putting a
// reverse proxy in front means switching to ClientIP() and setting trusted
// proxies in the same change, never separately.
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	l := &limiter{
		hits:   make(map[string][]time.Time),
		limit:  limit,
		window: window,
	}
	retryAfter := strconv.Itoa(int(window.Seconds()))

	return func(c *gin.Context) {
		if !l.allow(c.RemoteIP()) {
			c.Header("Retry-After", retryAfter)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

func (l *limiter) allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.calls++
	if l.calls%sweepEvery == 0 {
		l.evictIdle(cutoff)
	}

	recent := make([]time.Time, 0, len(l.hits[key])+1)
	for _, at := range l.hits[key] {
		if at.After(cutoff) {
			recent = append(recent, at)
		}
	}

	if len(recent) >= l.limit {
		l.hits[key] = recent
		return false
	}

	l.hits[key] = append(recent, now)
	return true
}

func (l *limiter) evictIdle(cutoff time.Time) {
	for key, hits := range l.hits {
		if len(hits) == 0 || hits[len(hits)-1].Before(cutoff) {
			delete(l.hits, key)
		}
	}
}
