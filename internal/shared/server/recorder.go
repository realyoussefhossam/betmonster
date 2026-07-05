package server

import "net/http"

// responseRecorder captures the status code of an HTTP response.
// It defaults to 200 OK if WriteHeader is never called, which matches
// the behavior of net/http.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	wrote      bool
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	if !r.wrote {
		r.ResponseWriter.WriteHeader(statusCode)
		r.statusCode = statusCode
		r.wrote = true
	}
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}
