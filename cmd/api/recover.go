package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover returns middleware that recovers from panics in downstream handlers.
// It logs the panic value and stack, then writes HTTP 500. Without this hook,
// a single panic would terminate the process. Safe to wrap the root ServeMux
// so every route, including /livez and /readyz, is covered.
//
// Phase 4 (M6): when RequestLog wraps the ResponseWriter, skip http.Error if
// the response has already started (see comment below).
func Recover(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log for operators; never send panic text or stack to clients in prod.
				logger.Error("panic",
					"error", fmt.Sprint(err),
					"stack", string(debug.Stack()),
					"path", r.URL.Path,
					"method", r.Method,
				)
				// If headers already sent, you cannot change status; http.Error may still write body.
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
