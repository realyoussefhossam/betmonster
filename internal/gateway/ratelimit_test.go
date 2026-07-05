package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
	req = req.WithContext(context.WithValue(req.Context(), "user", auth.User{ID: "a"}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// User "a" second request blocked.
	req = httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user", auth.User{ID: "a"}))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	// User "b" first request allowed because they have a separate limit.
	req = httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), "user", auth.User{ID: "b"}))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}
