package main

import (
	"integration-engine/internal/config"
	"net/http"
)

type App struct {
	cfg *config.Config
}

func NewApp(cfg *config.Config) *App {
	return &App{cfg}
}

// routes registers all HTTP handlers on a ServeMux. Tests and production use the same wiring.
// Stripe signature verification will use app.cfg in a later branch; see PLAN.md Milestone 2.
func (app *App) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", app.handleLivez)
	mux.HandleFunc("GET /readyz", app.handleReadyz)
	mux.HandleFunc("POST /webhooks/stripe", app.handleStripeWebhook)
	return mux
}
