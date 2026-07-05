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

const windowSeconds = 60

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
			Addr:         redisAddr,
			PoolSize:     100,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			MaxRetries:   3,
		})
	}

	return &RateLimiter{
		backend: backend,
		redis:   redisClient,
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    burst,
	}
}

// Ping verifies the Redis connection is reachable. It is a no-op for the memory backend.
func (rl *RateLimiter) Ping(ctx context.Context) error {
	if rl.redis == nil {
		return nil
	}
	return rl.redis.Ping(ctx).Err()
}

// Close closes the Redis connection. It is a no-op for the memory backend.
func (rl *RateLimiter) Close() error {
	if rl.redis == nil {
		return nil
	}
	return rl.redis.Close()
}

// Middleware returns an HTTP handler that applies rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rl.key(r)
		allowed, err := rl.allow(r.Context(), key)
		if err != nil {
			w.Header().Set("Retry-After", strconv.Itoa(windowSeconds))
			http.Error(w, "rate limiter unavailable", http.StatusTooManyRequests)
			return
		}
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(windowSeconds))
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
	if user != nil {
		if u, ok := user.(auth.User); ok {
			return fmt.Sprintf("%s:%s", ip, u.ID)
		}
		return fmt.Sprintf("%s:%v", ip, user)
	}
	return ip
}

// clientIP returns the client IP. If an X-Forwarded-For header is present, the
// rightmost IP is used (the one closest to the gateway). This is intended for
// deployments where the gateway runs behind a trusted reverse proxy. If no
// trusted proxy is configured, use the RemoteAddr directly.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		last := strings.TrimSpace(parts[len(parts)-1])
		if last != "" {
			return last
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
	rl.mu.RUnlock()
	if ok {
		return lim
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()
	lim, ok = rl.limiters[key]
	if !ok {
		lim = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[key] = lim
	}
	return lim
}

// allowRedis uses a token bucket stored in Redis. The bucket is refilled at
// rl.rate tokens per second and capped at rl.burst tokens. Timestamps are
// seconds since epoch, so the refill rate is rate tokens per second.
func (rl *RateLimiter) allowRedis(ctx context.Context, key string) (bool, error) {
	now := time.Now().Unix()

	lua := `
		local key = KEYS[1]
		local now = tonumber(ARGV[1])
		local rate = tonumber(ARGV[2])
		local burst = tonumber(ARGV[3])

		local state = redis.call('HMGET', key, 'tokens', 'ts')
		local tokens = state[1]
		local ts = state[2]

		if tokens == false then
			tokens = burst
			ts = now
		else
			tokens = tonumber(tokens)
			ts = tonumber(ts)
			local delta = (now - ts) * rate
			tokens = math.min(burst, tokens + delta)
		end

		if tokens >= 1 then
			tokens = tokens - 1
			redis.call('HSET', key, 'tokens', tokens, 'ts', now)
			redis.call('EXPIRE', key, 120)
			return 1
		end

		redis.call('HSET', key, 'tokens', tokens, 'ts', now)
		redis.call('EXPIRE', key, 120)
		return 0
	`

	res, err := rl.redis.Eval(ctx, lua, []string{key}, now, float64(rl.rate), rl.burst).Result()
	if err != nil {
		return false, err
	}
	allow, ok := res.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected redis response type")
	}
	return allow == 1, nil
}
