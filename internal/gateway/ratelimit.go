package gateway

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/realyoussefhossam/betmonster/internal/auth"
	"golang.org/x/time/rate"
)

const (
	retryAfterSeconds = 60
	windowSeconds     = 60
)

// userKey is the context key used to store the authenticated user.
type userKey struct{}

var UserContextKey = userKey{}

// RateLimiter enforces per-IP and per-user rate limits using either an
// in-process token bucket or a Redis-backed sliding window.
type RateLimiter struct {
	backend string
	redis   *redis.Client

	mu       sync.RWMutex
	limiters map[string]*rate.Limiter

	rate  rate.Limit
	burst int

	maxWindow int
}

// NewRateLimiter creates a rate limiter for the given backend.
// Supported backends are "memory" and "redis". For the memory backend,
// redisAddr may be empty. rps and burst default to 100 when non-positive.
func NewRateLimiter(backend, redisAddr string, rps, burst int) *RateLimiter {
	if rps <= 0 {
		rps = 100
	}
	if burst <= 0 {
		burst = rps
	}

	var redisClient *redis.Client
	if backend == "redis" {
		if redisAddr == "" {
			redisAddr = "redis:6379"
		}
		redisClient = redis.NewClient(&redis.Options{
			Addr: redisAddr,
		})
	}

	return &RateLimiter{
		backend: backend,
		redis:   redisClient,
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    burst,
		maxWindow: rps * windowSeconds,
	}
}

// Middleware returns an HTTP handler that applies rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rl.key(r)
		allowed, err := rl.allow(r.Context(), key)
		if err != nil {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
			http.Error(w, "rate limiter unavailable", http.StatusTooManyRequests)
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// key derives the rate limit key from the client IP and optional user identity.
func (rl *RateLimiter) key(r *http.Request) string {
	ip := clientIP(r)
	user := r.Context().Value(UserContextKey)
	if user == nil {
		user = r.Context().Value("user")
	}
	if user == nil {
		user = r.Context().Value("userID")
	}
	if user != nil {
		if u, ok := user.(auth.User); ok {
			return fmt.Sprintf("%s:%s", ip, u.ID)
		}
		return fmt.Sprintf("%s:%v", ip, user)
	}
	return ip
}

// clientIP returns the client IP from X-Forwarded-For or RemoteAddr.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		first := strings.Split(fwd, ",")[0]
		first = strings.TrimSpace(first)
		if first != "" {
			return first
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// allow reports whether the request identified by key should be allowed.
func (rl *RateLimiter) allow(ctx context.Context, key string) (bool, error) {
	if rl.backend == "redis" {
		return rl.allowRedis(ctx, key)
	}
	return rl.allowMemory(ctx, key), nil
}

// allowMemory uses a per-key token bucket stored in an in-process map.
func (rl *RateLimiter) allowMemory(ctx context.Context, key string) bool {
	_ = ctx
	lim := rl.getLimiter(key)
	return lim.Allow()
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.RLock()
	lim, ok := rl.limiters[key]
	if ok {
		defer rl.mu.RUnlock()
		return lim
	}
	rl.mu.RUnlock()

	rl.mu.Lock()
	defer rl.mu.Unlock()
	lim, ok = rl.limiters[key]
	if !ok {
		lim = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[key] = lim
	}
	return lim
}

// allowRedis uses a sliding-window counter in Redis.
// The window is 60 seconds and the maximum number of requests in that window
// is rps * 60.
func (rl *RateLimiter) allowRedis(ctx context.Context, key string) (bool, error) {
	now := time.Now().Unix()
	windowStart := now - windowSeconds

	lua := `
		redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, ARGV[2])
		local count = redis.call('ZCARD', KEYS[1])
		if count < tonumber(ARGV[3]) then
			redis.call('ZADD', KEYS[1], ARGV[1], ARGV[1])
			redis.call('EXPIRE', KEYS[1], 60)
			return 1
		end
		return 0
	`

	res, err := rl.redis.Eval(ctx, lua, []string{key}, now, windowStart, rl.maxWindow).Result()
	if err != nil {
		return false, err
	}
	allow, ok := res.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected redis response type")
	}
	return allow == 1, nil
}
