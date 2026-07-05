package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/realyoussefhossam/betmonster/internal/auth"
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

	// First request allowed.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	// Second request blocked.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "60", rec.Header().Get("Retry-After"))
}

func TestRateLimitRespectsExistingUserID(t *testing.T) {
	limiter := NewRateLimiter("memory", "", 1, 1)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// User "a" consumes their single token.
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, auth.User{ID: "a"}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// User "a" second request blocked.
	req = httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, auth.User{ID: "a"}))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	// User "b" first request allowed because they have a separate limit.
	req = httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, auth.User{ID: "b"}))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRateLimitRedisBackend(t *testing.T) {
	srv := miniredis.RunT(t)
	defer srv.Close()

	limiter := NewRateLimiter("redis", srv.Addr(), 2, 2)
	defer limiter.Close()
	assert.NoError(t, limiter.Ping(context.Background()))

	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		assert.Equal(t, http.StatusOK, rec.Code, "request %d", i+1)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}

func TestRateLimitRedisBackendUserIsolation(t *testing.T) {
	srv := miniredis.RunT(t)
	defer srv.Close()

	limiter := NewRateLimiter("redis", srv.Addr(), 1, 1)
	defer limiter.Close()
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, auth.User{ID: "a"}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	req = httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, auth.User{ID: "a"}))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	req = httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, auth.User{ID: "b"}))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestClientIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	assert.Equal(t, "192.0.2.1", clientIP(req))

	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
	assert.Equal(t, "10.0.0.2", clientIP(req))

	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	req.Header.Set("X-Forwarded-For", " 10.0.0.1 ")
	assert.Equal(t, "10.0.0.1", clientIP(req))

	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "invalid"
	assert.Equal(t, "invalid", clientIP(req))
}

func TestRateLimitKey(t *testing.T) {
	limiter := NewRateLimiter("memory", "", 10, 10)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	assert.Equal(t, "192.0.2.1", limiter.key(req))

	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:1234"
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, auth.User{ID: "user-1"}))
	assert.Equal(t, "192.0.2.1:user-1", limiter.key(req))
}

func TestRateLimitPingMemory(t *testing.T) {
	limiter := NewRateLimiter("memory", "", 10, 10)
	assert.NoError(t, limiter.Ping(context.Background()))
	assert.NoError(t, limiter.Close())
}
