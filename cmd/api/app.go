package main

import (
	"integration-engine/internal/config"
	"integration-engine/internal/store"
	"log/slog"
	"net/http"
)

type App struct {
	cfg    *config.Config
	logger *slog.Logger
	store  store.Store
}

func NewApp(cfg *config.Config, logger *slog.Logger, st store.Store) *App {
	return &App{cfg: cfg, logger: logger, store: st}
}

// routes registers all HTTP handlers on a ServeMux. Tests and production use the same wiring.
func (app *App) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", app.handleLivez)
	mux.HandleFunc("GET /readyz", app.handleReadyz)
	mux.HandleFunc("POST /webhooks/stripe", app.handleStripeWebhook)
	return mux
}
