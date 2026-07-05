package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const metricsNamespace = "betmonster"

var httpRequestsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: metricsNamespace,
		Name:      "http_requests_total",
		Help:      "Total HTTP requests by method, route, status",
	},
	[]string{"method", "route", "status"},
)

var httpRequestDuration = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: metricsNamespace,
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request duration in seconds",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	},
	[]string{"method", "route"},
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
}

func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := newResponseRecorder(w)

		next.ServeHTTP(rec, r)

		status := strconv.Itoa(rec.statusCode)
		route := GetRoutePattern(r.Context())
		if route == "" {
			// Fall back to the path-only portion of the URL, excluding query
			// strings. This is safe for the current gateway routes because they
			// contain no path parameters; callers should set a route pattern
			// for dynamic routes to avoid high cardinality labels.
			route = r.URL.Path
		}

		httpRequestsTotal.WithLabelValues(r.Method, route, status).Inc()
		httpRequestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())
	})
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
