package main

import (
	"log/slog"
	"net/http"
	"time"
)

func isProbeRequest(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	path := r.URL.Path
	return path == "/livez" || path == "/readyz"
}

type responseRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func (w *responseRecorder) WriteHeader(code int) {
	w.status = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseRecorder) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// RequestLog returns middleware that assigns a request ID, attaches a request-scoped
// logger on context, and logs request_started / request_completed (skipped for probes).
func RequestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Step 1: Add "time" import; record start for duration_ms on request_completed.
		start := time.Now()

		// Step 2: Resolve ID (X-Request-Id header or UUID) via logging.go.
		id := resolveRequestID(r)

		// Step 3: Build request-scoped logger so every log line includes request_id.
		reqLogger := logger.With("request_id", id)

		// Step 4: Put request ID and logger on context for handlers (loggerFromContext).
		r = r.WithContext(withRequestContext(r.Context(), id, reqLogger))

		// Step 5: Access log — start (skip for GET /livez and GET /readyz).
		if !isProbeRequest(r) {
			reqLogger.Info(msgRequestStarted,
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)
		}

		// Step 6: Wrap ResponseWriter so we capture status (and bytes) after the handler runs.
		rec := &responseRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		// Step 7: Run the route handler (mux → handleStripeWebhook, probes, etc.).
		next.ServeHTTP(rec, r)

		// Step 8: Access log — end (skip probes). Use rec.status and time.Since(start).
		if !isProbeRequest(r) {
			reqLogger.Info(msgRequestCompleted,
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		}
	})
}
