package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func resetMetrics() {
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()
}

func TestMetricsEndpoint(t *testing.T) {
	resetMetrics()
	defer resetMetrics()

	mux := http.NewServeMux()
	mux.Handle("/metrics", MetricsHandler())
	mux.Handle("/test", WithRoutePattern("/test", Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))))

	// Trigger a request to generate metrics.
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify metrics endpoint exposes the recorded request.
	req = httptest.NewRequest("GET", "/metrics", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.True(t, strings.Contains(body, "betmonster_http_requests_total"))
	assert.True(t, strings.Contains(body, "method=\"GET\""))
	assert.True(t, strings.Contains(body, "route=\"/test\""))
	assert.True(t, strings.Contains(body, "status=\"200\""))
}

func TestMetricsMiddlewareRecordsStatusCode(t *testing.T) {
	resetMetrics()
	defer resetMetrics()

	mux := http.NewServeMux()
	mux.Handle("/metrics", MetricsHandler())
	mux.Handle("/created", WithRoutePattern("/created", Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/created", nil))
	assert.Equal(t, http.StatusCreated, rec.Code)

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, strings.Contains(rec.Body.String(), "betmonster_http_requests_total{method=\"POST\",route=\"/created\",status=\"201\"}"))
}

func TestMetricsMiddlewareRecordsDefaultStatus(t *testing.T) {
	resetMetrics()
	defer resetMetrics()

	mux := http.NewServeMux()
	mux.Handle("/metrics", MetricsHandler())
	mux.Handle("/write", WithRoutePattern("/write", Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/write", nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, strings.Contains(rec.Body.String(), "status=\"200\""))
}

func TestMetricsMiddlewareUsesFallbackPath(t *testing.T) {
	resetMetrics()
	defer resetMetrics()

	mux := http.NewServeMux()
	mux.Handle("/metrics", MetricsHandler())
	mux.Handle("/fallback", Metrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/fallback?currency=USDT", nil))
	assert.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, strings.Contains(rec.Body.String(), "route=\"/fallback\""))
}

func TestPrometheusNamespace(t *testing.T) {
	assert.Equal(t, "betmonster", metricsNamespace)
}

func TestMetricsHandler(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", MetricsHandler())

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain; version=0.0.4"))
}
