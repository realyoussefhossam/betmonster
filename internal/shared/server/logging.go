package server

import (
	"log/slog"
	"net/http"
	"time"
)

func Logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := GetRequestID(r.Context())
		rec := newResponseRecorder(w)

		next.ServeHTTP(rec, r)

		logger.Info(
			"handled request",
			slog.String("requestID", id),
			slog.Int("statusCode", rec.statusCode),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("userAgent", r.UserAgent()),
			slog.String("remoteAddr", r.RemoteAddr),
			slog.Any("duration", time.Since(start)),
		)
	})
}
