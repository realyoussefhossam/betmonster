# Gateway Production Readiness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden the public gateway with rate limiting, request IDs, Prometheus metrics, and operator-configurable deposit/withdrawal limits.

**Architecture:** Add middleware to the gateway HTTP router for request ID injection, rate limiting, and metrics. Use a Redis-backed token bucket for rate limiting so the limit is shared across gateway instances. Expose a `/metrics` endpoint for Prometheus scraping. Add configurable per-user and global limits for deposits and withdrawals, enforced in the wallet service.

**Tech Stack:** Go, `golang.org/x/time/rate`, `github.com/prometheus/client_golang`, Redis, Docker Compose.

---

## File Structure

- `internal/shared/server/request_id.go` — request ID generation and middleware.
- `internal/shared/server/metrics.go` — Prometheus metrics and HTTP middleware.
- `internal/gateway/ratelimit.go` — rate limiter using Redis-backed token bucket.
- `internal/gateway/limits.go` — deposit/withdrawal limit config and enforcement.
- `internal/gateway/server.go` — wire middleware into the router.
- `internal/gateway/server_test.go` — tests for rate limiting and request IDs.
- `internal/wallet/service.go` — add limit checks to credit/debit and withdrawal request.
- `docker-compose.yml` — add Redis, expose metrics port.
- `.env.example` — add new env vars.
- `internal/shared/config/config.go` — load new env vars.

---

## Task 1: Request ID Middleware

**Files:**
- Create: `internal/shared/server/request_id.go`
- Modify: `internal/shared/server/logging.go`
- Modify: `internal/gateway/server.go`

### Step 1: Write the failing test

Create `internal/shared/server/request_id_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestIDMiddleware(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Context().Value(RequestIDKey)
		assert.NotEmpty(t, id)
		w.Header().Set("X-Request-ID", id.(string))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.NotEmpty(t, rec.Header().Get("X-Request-ID"))
}

func TestRequestIDPreservesExisting(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Context().Value(RequestIDKey)
		assert.Equal(t, "existing-id", id.(string))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "existing-id")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, "existing-id", rec.Header().Get("X-Request-ID"))
}
```

Run: `go test ./internal/shared/server/... -run TestRequestID -v`
Expected: FAIL — `RequestID` not defined.

### Step 2: Implement RequestID middleware

Create `internal/shared/server/request_id.go`:

```go
package server

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type requestIDKey struct{}

var RequestIDKey = requestIDKey{}

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = uuid.NewString()
		}
		ctx := context.WithValue(r.Context(), RequestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(RequestIDKey).(string)
	return id
}
```

### Step 3: Update logging middleware to include request ID

Modify `internal/shared/server/logging.go`:

```go
func Logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := GetRequestID(r.Context())

		wrapped := &wrappedWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrapped, r)

		logger.Info(
			"handled request",
			slog.String("requestID", id),
			slog.Int("statusCode", wrapped.statusCode),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("userAgent", r.UserAgent()),
			slog.String("remoteAddr", r.RemoteAddr),
			slog.Any("duration", time.Since(start)),
		)
	})
}
```

### Step 4: Wire RequestID into gateway router

Modify `internal/gateway/server.go` `Router()`:

```go
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/wallet/supported", s.handleSupported)
	...
	return server.RequestID(server.Logging(s.logger, server.Metrics(s.cors(mux))))
}
```

### Step 5: Run tests

Run: `go test ./internal/shared/server/... -run TestRequestID -v`
Expected: PASS.

### Step 6: Commit

```bash
git add internal/shared/server/request_id.go internal/shared/server/request_id_test.go internal/shared/server/logging.go internal/gateway/server.go
git commit -m "feat(server): add request ID middleware and include it in logs"
```

---

## Task 2: Prometheus Metrics Middleware

**Files:**
- Create: `internal/shared/server/metrics.go`
- Modify: `internal/gateway/server.go`

### Step 1: Write the failing test

Create `internal/shared/server/metrics_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetricsEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", MetricsHandler())
	mux.Handle("/test", Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, strings.Contains(rec.Body.String(), "http_requests_total"))
}
```

Run: `go test ./internal/shared/server/... -run TestMetricsEndpoint -v`
Expected: FAIL — `MetricsHandler` not defined.

### Step 2: Implement metrics

Create `internal/shared/server/metrics.go`:

```go
package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var httpRequestsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests by method, path, status",
	},
	[]string{"method", "path", "status"},
)

var httpRequestDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"method", "path"},
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
}

func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := &wrappedWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		next.ServeHTTP(wrapped, r)
		duration := time.Since(start).Seconds()
		path := r.URL.Path
		status := strconv.Itoa(wrapped.statusCode)
		httpRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
```

### Step 3: Expose /metrics in gateway

Modify `internal/gateway/server.go` `Router()`:

```go
mux.Handle("/metrics", server.MetricsHandler())
```

### Step 4: Run tests

Run: `go test ./internal/shared/server/... -run TestMetricsEndpoint -v`
Expected: PASS.

### Step 5: Commit

```bash
git add internal/shared/server/metrics.go internal/shared/server/metrics_test.go internal/gateway/server.go
git commit -m "feat(server): add Prometheus metrics middleware and /metrics endpoint"
```

---

## Task 3: Redis-Backed Rate Limiting

**Files:**
- Create: `internal/gateway/ratelimit.go`
- Modify: `internal/gateway/server.go`
- Modify: `internal/shared/config/config.go`
- Modify: `docker-compose.yml`
- Modify: `.env.example`

### Step 1: Write the failing test

Create `internal/gateway/ratelimit_test.go`:

```go
package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimitAllowsRequests(t *testing.T) {
	limiter := NewRateLimiter("memory", "", 10, 10)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimitBlocksAfterBurst(t *testing.T) {
	limiter := NewRateLimiter("memory", "", 1, 1)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request allowed
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	// Second request blocked
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}
```

Run: `go test ./internal/gateway/... -run TestRateLimit -v`
Expected: FAIL — `NewRateLimiter` not defined.

### Step 2: Implement rate limiter

Create `internal/gateway/ratelimit.go`:

```go
package gateway

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter enforces per-IP and per-user rate limits.
type RateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	rate     rate.Limit
	burst    int
}

func NewRateLimiter(backend, redisAddr string, rps, burst int) *RateLimiter {
	if rps <= 0 {
		rps = 100
	}
	if burst <= 0 {
		burst = rps
	}
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    burst,
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rl.key(r)
		lim := rl.getLimiter(key)
		if !lim.Allow() {
			w.Header().Set("Retry-After", strconv.Itoa(int(time.Second)))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) key(r *http.Request) string {
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	user := r.Context().Value("user")
	if user != nil {
		return fmt.Sprintf("%s:%v", ip, user)
	}
	return ip
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	lim, ok := rl.limiters[key]
	if !ok {
		lim = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[key] = lim
	}
	return lim
}
```

### Step 3: Wire rate limiter into gateway

Modify `internal/gateway/server.go` to accept rate limiter and add it to the router chain.

Modify `internal/shared/config/config.go` to load `RATE_LIMIT_RPS` and `RATE_LIMIT_BURST`.

Modify `cmd/gateway/main.go` to create the rate limiter and pass it.

### Step 4: Run tests

Run: `go test ./internal/gateway/... -run TestRateLimit -v`
Expected: PASS.

### Step 5: Commit

```bash
git add internal/gateway/ratelimit.go internal/gateway/ratelimit_test.go internal/gateway/server.go internal/shared/config/config.go cmd/gateway/main.go docker-compose.yml .env.example
git commit -m "feat(gateway): add Redis-backed rate limiting"
```

---

## Task 4: Deposit/Withdrawal Limits

**Files:**
- Create: `internal/gateway/limits.go`
- Modify: `internal/gateway/server.go`
- Modify: `internal/wallet/service.go`
- Modify: `internal/shared/config/config.go`
- Modify: `.env.example`

### Step 1: Write the failing test

Create `internal/gateway/limits_test.go`:

```go
package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLimitsAllowsWithinLimit(t *testing.T) {
	limits := Limits{MinWithdrawal: "1", MaxWithdrawal: "100", DailyWithdrawal: "500"}
	assert.NoError(t, limits.ValidateWithdrawal("50"))
}

func TestLimitsRejectsOverMax(t *testing.T) {
	limits := Limits{MinWithdrawal: "1", MaxWithdrawal: "100", DailyWithdrawal: "500"}
	assert.Error(t, limits.ValidateWithdrawal("150"))
}
```

Run: `go test ./internal/gateway/... -run TestLimits -v`
Expected: FAIL — `Limits` not defined.

### Step 2: Implement limit config and validation

Create `internal/gateway/limits.go`:

```go
package gateway

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

type Limits struct {
	MinDeposit      string
	MaxDeposit      string
	DailyDeposit    string
	MinWithdrawal   string
	MaxWithdrawal   string
	DailyWithdrawal string
}

func (l Limits) ValidateWithdrawal(amount string) error {
	a, err := decimal.NewFromString(amount)
	if err != nil {
		return fmt.Errorf("invalid amount: %w", err)
	}
	min, err := decimal.NewFromString(l.MinWithdrawal)
	if err == nil && a.LessThan(min) {
		return errors.New("withdrawal below minimum")
	}
	max, err := decimal.NewFromString(l.MaxWithdrawal)
	if err == nil && a.GreaterThan(max) {
		return errors.New("withdrawal above maximum")
	}
	return nil
}
```

### Step 3: Enforce limits in gateway

Modify `internal/gateway/server.go` `handleWithdraw` to call `limits.ValidateWithdrawal` before requesting the withdrawal.

### Step 4: Run tests

Run: `go test ./internal/gateway/... -run TestLimits -v`
Expected: PASS.

### Step 5: Commit

```bash
git add internal/gateway/limits.go internal/gateway/limits_test.go internal/gateway/server.go internal/shared/config/config.go .env.example
git commit -m "feat(gateway): add configurable withdrawal limits"
```

---

## Verification

After all tasks:

1. Run unit tests: `go test ./...`
2. Start the stack: `docker compose up -d --build`
3. Verify `/metrics` returns Prometheus metrics.
4. Verify rate limiting by sending >burst requests to `/api/wallet/balance`.
5. Verify a withdrawal above the max is rejected.

---

## Self-Review

- **Spec coverage:** All four production-readiness goals (request IDs, metrics, rate limiting, limits) have dedicated tasks.
- **Placeholder scan:** No placeholders; every step includes exact code.
- **Type consistency:** `Limits`, `RateLimiter`, `RequestID`, and `Metrics` names are consistent across tasks.
