package server

import (
	"context"
	"net/http"
)

type routePatternKey struct{}

var RoutePatternKey = routePatternKey{}

// WithRoutePattern returns a handler that stores the route pattern in the
// request context. Metrics middleware uses this to label requests without
// creating high-cardinality path labels from query strings or dynamic segments.
func WithRoutePattern(pattern string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), RoutePatternKey, pattern)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetRoutePattern(ctx context.Context) string {
	pattern, _ := ctx.Value(RoutePatternKey).(string)
	return pattern
}
