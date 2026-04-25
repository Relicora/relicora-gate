package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
	"time"
)

// RequestLogger logs each incoming request and the response status.
// If logger is nil, log.Default() is used.
func RequestLogger(logger *log.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = log.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			logger.Printf("[HTTP] %s %s %s", r.RemoteAddr, r.Method, r.URL.Path)
			lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(lrw, r)
			duration := time.Since(start)
			logger.Printf("[HTTP] %s %s %s -> %d duration=%s", r.RemoteAddr, r.Method, r.URL.Path, lrw.statusCode, duration)
		})
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (l *loggingResponseWriter) WriteHeader(code int) {
	l.statusCode = code
	l.ResponseWriter.WriteHeader(code)
}

// Recoverer recovers from panics in downstream handlers and returns HTTP 500.
// If logger is nil, log.Default() is used.
func Recoverer(logger *log.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = log.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Printf("[RECOVER] panic recovered: %v\n%s", rec, debug.Stack())
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Timeout wraps a handler with a request timeout.
// If the handler does not complete before the duration, 503 Service Unavailable is returned.
func Timeout(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, "request timed out")
	}
}
