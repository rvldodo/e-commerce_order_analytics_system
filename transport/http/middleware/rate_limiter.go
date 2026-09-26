package middleware

import (
	"e-commerce_order_analytics_system/pkg/logger"
	"e-commerce_order_analytics_system/pkg/response"
	"math"
	"strconv"
	"sync"
	"time"

	apperror "e-commerce_order_analytics_system/pkg/errors"
	"github.com/gin-gonic/gin"
)

type RateLimitConfig struct {
	Requests      int
	Window        time.Duration
	Burst         int
	KeyFunc       func(c *gin.Context) string
	Skip          func(c *gin.Context) bool
	IdleTTL       time.Duration
	SweepInterval time.Duration
}

func (cfg *RateLimitConfig) applyDefaults() {
	if cfg.Requests <= 0 {
		cfg.Requests = 100
	}
	if cfg.Window <= 0 {
		cfg.Window = time.Minute
	}
	if cfg.Burst <= 0 {
		cfg.Burst = cfg.Requests
	}
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(c *gin.Context) string { return c.ClientIP() }
	}
	if cfg.IdleTTL <= 0 {
		cfg.IdleTTL = 10 * cfg.Window
		if cfg.IdleTTL < time.Minute {
			cfg.IdleTTL = time.Minute
		}
	}
	if cfg.SweepInterval <= 0 {
		cfg.SweepInterval = cfg.IdleTTL
	}
}

type bucket struct {
	tokens float64
	last   time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket

	refill float64 // tokens per second
	burst  float64
	cfg    RateLimitConfig

	stop     chan struct{}
	stopOnce sync.Once
}

func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	cfg.applyDefaults()

	rl := &RateLimiter{
		buckets: make(map[string]*bucket),
		refill:  float64(cfg.Requests) / cfg.Window.Seconds(),
		burst:   float64(cfg.Burst),
		cfg:     cfg,
		stop:    make(chan struct{}),
	}

	go rl.sweeper()
	return rl
}

func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() { close(rl.stop) })
}

func (rl *RateLimiter) sweeper() {
	t := time.NewTicker(rl.cfg.SweepInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			rl.evict(time.Now())
		case <-rl.stop:
			return
		}
	}
}

func (rl *RateLimiter) evict(now time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for k, b := range rl.buckets {
		if now.Sub(b.last) > rl.cfg.IdleTTL {
			delete(rl.buckets, k)
		}
	}
}

func (rl *RateLimiter) Allow(key string) (ok bool, remaining int, retryAfter time.Duration) {
	return rl.allowAt(key, time.Now())
}

func (rl *RateLimiter) allowAt(key string, now time.Time) (bool, int, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, seen := rl.buckets[key]
	if !seen {
		b = &bucket{tokens: rl.burst, last: now}
		rl.buckets[key] = b
	} else {
		elapsed := now.Sub(b.last).Seconds()
		if elapsed > 0 {
			b.tokens = math.Min(rl.burst, b.tokens+elapsed*rl.refill)
			b.last = now
		}
	}

	if b.tokens < 1 {
		deficit := 1 - b.tokens
		return false, 0, time.Duration(deficit / rl.refill * float64(time.Second))
	}

	b.tokens--
	return true, int(b.tokens), 0
}

func RateLimit(cfg RateLimitConfig) (gin.HandlerFunc, *RateLimiter) {
	rl := NewRateLimiter(cfg)
	return rl.Middleware(), rl
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	limit := strconv.Itoa(rl.cfg.Requests)

	return func(c *gin.Context) {
		if rl.cfg.Skip != nil && rl.cfg.Skip(c) {
			c.Next()
			return
		}

		key := rl.cfg.KeyFunc(c)
		if key == "" {
			key = "-"
		}

		ok, remaining, retryAfter := rl.Allow(key)

		c.Header("X-RateLimit-Limit", limit)
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if ok {
			c.Next()
			return
		}

		secs := int(math.Ceil(retryAfter.Seconds()))
		if secs < 1 {
			secs = 1
		}
		c.Header("Retry-After", strconv.Itoa(secs))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(retryAfter).Unix(), 10))

		if logger.Log != nil {
			logger.SugarWithContext(c.Request.Context()).Warnw("rate limit exceeded",
				"key", key,
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"retryAfterSeconds", secs,
			)
		}

		// Reuse the registered code so the body matches every other error the
		// API returns, rather than inventing a second 429 shape.
		response.Err(c, apperror.New(apperror.RateLimited))
		c.Abort()
	}
}

// ── common key functions ──────────────────────────────────────────────────────

// KeyByIP limits per client IP. This is gin's ClientIP, so it honours
// X-Forwarded-For only for proxies you have marked trusted via
// (*gin.Engine).SetTrustedProxies. Leave that unset behind a load balancer and
// every request appears to come from the balancer, collapsing all clients into
// one bucket.
func KeyByIP() func(*gin.Context) string {
	return func(c *gin.Context) string { return c.ClientIP() }
}

// KeyByHeader limits per header value — an API key or device token. Falls back
// to the client IP when the header is absent, so an unauthenticated caller
// cannot dodge the limit by omitting it.
func KeyByHeader(name string) func(*gin.Context) string {
	return func(c *gin.Context) string {
		if v := c.GetHeader(name); v != "" {
			return name + ":" + v
		}
		return "ip:" + c.ClientIP()
	}
}

// KeyByContext limits per value previously stored in the gin context — the
// authenticated user or tenant set by auth middleware. Falls back to the
// client IP when the key is missing or not a string.
func KeyByContext(ctxKey string) func(*gin.Context) string {
	return func(c *gin.Context) string {
		if v, exists := c.Get(ctxKey); exists {
			if s, isStr := v.(string); isStr && s != "" {
				return ctxKey + ":" + s
			}
		}
		return "ip:" + c.ClientIP()
	}
}
