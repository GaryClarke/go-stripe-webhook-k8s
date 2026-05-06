package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"
)

// healthResponse is used for probe-style JSON bodies (livez, readyz).
type healthResponse struct {
	Status string `json:"status"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		resp := healthResponse{Status: "ok"}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("livez: encode response: %v", err)
		}
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		resp := healthResponse{Status: "ok"}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("readyz: encode response: %v", err)
		}
	})

	addr := ":8080"
	handler := Recover(mux)

	// http.Server is the long-lived server value. Using it (instead of
	// http.ListenAndServe alone) gives us Shutdown(), which Kubernetes expects
	// when the pod gets SIGTERM: drain in-flight work, then exit cleanly.
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// ListenAndServe blocks for the whole lifetime of the server. If we called it
	// in main alone, we would never reach the signal handler or Shutdown below.
	// Running it in a goroutine lets main wait on OS signals while the server runs.
	go func() {
		log.Printf("listening on %s", srv.Addr)
		err := srv.ListenAndServe()
		// After Shutdown(), ListenAndServe returns http.ErrServerClosed. That is
		// expected. Any other error (e.g. address already in use) is fatal.
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	// Signal channel: buffer 1 so one notification can be queued before <-quit runs
	// (see os/signal: caller must ensure c can keep up; unbuffered can be awkward).
	quit := make(chan os.Signal, 1)

	// Register which signals we care about. This call returns immediately; it does
	// NOT wait for a signal. Blocking happens on <-quit below.
	// SIGINT  = terminal Ctrl+C locally.
	// SIGTERM = typical "please shut down" from Kubernetes and process managers.
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Block until one registered signal is delivered. We do not assign the value:
	// we only need "something arrived". You could use sig := <-quit if you want to
	// log which signal (SIGINT vs SIGTERM). For several channels at once, use select.
	<-quit

	log.Printf("shutting down...")

	// Shutdown stops accepting new connections and waits for in-flight requests to
	// finish, or until ctx times out. Align this timeout with pod terminationGracePeriodSeconds.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// If the deadline passes before work drains, err is often context.DeadlineExceeded.
	// We log rather than Fatalf so shutdown can complete; tune timeout or use a
	// non-zero exit in production if your platform cares.
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

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
