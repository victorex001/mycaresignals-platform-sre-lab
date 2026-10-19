// internal/middleware/ratelimit.go
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	mu       sync.Mutex
	visitors = map[string]*visitor{}
	onceGC   sync.Once
)

func getLimiter(ip string, rps float64, burst int) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	v, ok := visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(rate.Limit(rps), burst)}
		visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func gc() {
	for {
		time.Sleep(time.Minute)
		mu.Lock()
		for ip, v := range visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(visitors, ip)
			}
		}
		mu.Unlock()
	}
}

func NewRateLimiter(rps float64, burst int) gin.HandlerFunc {
	// Ensure GC starts only once
	onceGC.Do(func() { go gc() })

	return func(c *gin.Context) {
		// Don’t rate-limit OPTIONS or health checks
		if c.Request.Method == http.MethodOptions ||
			c.Request.URL.Path == "/livez" ||
			c.Request.URL.Path == "/readyz" {
			c.Next()
			return
		}

		if !getLimiter(c.ClientIP(), rps, burst).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}

		c.Next()
	}
}