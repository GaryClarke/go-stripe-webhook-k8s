package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"integration-engine/internal/config"
)

func main() {
	logger := NewJSONLogger(os.Stdout, nil)
	cfg, err := config.Load()
	if err != nil {
		logger.Error(msgConfigLoadFailed, "error", err.Error())
		os.Exit(1)
	}
	app := NewApp(cfg, logger)

	addr := ":" + cfg.Port
	handler := Recover(logger, RequestLog(logger, app.routes()))

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
		logger.Info(msgServerListening, "addr", srv.Addr)
		err := srv.ListenAndServe()
		// After Shutdown(), ListenAndServe returns http.ErrServerClosed. That is
		// expected. Any other error (e.g. address already in use) is fatal.
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(msgServerListenError,
				"error", err.Error(),
				"addr", srv.Addr,
			)
			os.Exit(1)
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
	// we only need "something arrived". Uses sig := <-quit to
	// log which signal (SIGINT vs SIGTERM). For several channels at once, use select.
	sig := <-quit
	logger.Info(msgShuttingDown, "signal", sig.String())

	// Shutdown stops accepting new connections and waits for in-flight requests to
	// finish, or until ctx times out. Align this timeout with pod terminationGracePeriodSeconds.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// If the deadline passes before work drains, err is often context.DeadlineExceeded.
	// We log rather than Fatalf so shutdown can complete; tune timeout or use a
	// non-zero exit in production if your platform cares.
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error(msgServerShutdownError,
			"error", err.Error(),
		)
	}
}
