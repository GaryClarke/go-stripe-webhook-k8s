package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/stripe/stripe-go/v85"
)

// healthResponse is used for probe-style JSON bodies (livez, readyz).
type healthResponse struct {
	Status string `json:"status"`
}

func (app *App) handleLivez(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	resp := healthResponse{Status: "ok"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		app.logger.Error("probe_encode_error",
			"probe", "livez",
			"error", err.Error(),
		)
	}
}

func (app *App) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	resp := healthResponse{Status: "ok"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		app.logger.Error("probe_encode_error",
			"probe", "readyz",
			"error", err.Error(),
		)
	}
}

const maxStripeWebhookBody = 1 << 20 // 1 MiB; caps oversized abuse while fitting real Stripe events

func (app *App) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxStripeWebhookBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			app.logger.Error("stripe_body_too_large",
				"max_bytes", maxStripeWebhookBody,
			)
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		}
		app.logger.Error("stripe_event_verify_failed",
			"reason", "read_body",
			"error", err.Error(),
		)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	sigHeader := r.Header.Get("Stripe-Signature")
	ev, err := stripe.ConstructEvent(body, sigHeader, app.cfg.StripeWebhookSecret)
	if err != nil {
		// Stripe's webhook examples use 400 for invalid payload / signature verification failures.
		app.logger.Error("stripe_event_verify_failed",
			"reason", "verify_event",
			"error", err.Error(),
		)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	app.logger.Info("stripe_event_accepted",
		"event_id", ev.ID,
		"event_type", string(ev.Type),
		"body_bytes", len(body),
		"remote_addr", r.RemoteAddr,
	)

	w.WriteHeader(http.StatusNoContent)
}
