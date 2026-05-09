package main

import (
	"log"
	"net/http"
	"runtime/debug"
)

// Recover returns middleware that recovers from panics in downstream handlers.
// It logs the panic value and stack, then writes HTTP 500. Without this hook,
// a single panic would terminate the process. Safe to wrap the root ServeMux
// so every route, including /livez and /readyz, is covered.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log for operators; never send panic text or stack to clients in prod.
				log.Printf("panic: %v\n%s", err, debug.Stack())
				// If headers already sent, you cannot change status; http.Error may still write body.
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
