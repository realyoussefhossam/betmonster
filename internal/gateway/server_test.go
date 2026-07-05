package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/realyoussefhossam/betmonster/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestHealthHandler(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", Limits{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "healthy")
}

func TestSupportedOptions(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT,USDC", "anvil,base", Limits{})
	req := httptest.NewRequest(http.MethodGet, "/api/wallet/supported", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "USDT")
	assert.Contains(t, w.Body.String(), "USDC")
	assert.Contains(t, w.Body.String(), "anvil")
	assert.Contains(t, w.Body.String(), "base")
}

func TestHandleBalanceUnsupportedCurrency(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(logger, nil, nil, NewRateLimiter("memory", "", 100, 100), "", "", "USDT", "anvil", Limits{})
	req := httptest.NewRequest(http.MethodGet, "/api/wallet/balance?currency=BTC", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, auth.User{ID: "user-1"}))
	w := httptest.NewRecorder()

	srv.handleBalance(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "unsupported currency")
}
