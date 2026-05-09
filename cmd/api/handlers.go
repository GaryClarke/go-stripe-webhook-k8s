package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"integration-engine/internal/engine"
)

// healthResponse is used for probe-style JSON bodies (livez, readyz).
type healthResponse struct {
	Status string `json:"status"`
}

func (app *App) handleLivez(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	resp := healthResponse{Status: "ok"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("livez: encode response: %v", err)
	}
}

func (app *App) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	resp := healthResponse{Status: "ok"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("readyz: encode response: %v", err)
	}
}

const maxStripeWebhookBody = 1 << 20 // 1 MiB; caps oversized abuse while fitting real Stripe events

func (app *App) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxStripeWebhookBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			log.Printf("webhooks/stripe: body over max (%d bytes)", maxStripeWebhookBody)
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		}
		log.Printf("webhooks/stripe: read body: %v", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	ev, err := engine.ParseStripeEvent(body)
	if err != nil {
		log.Printf("webhooks/stripe: %v", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Log whether the Stripe-Signature header is present; never log the value (verification comes in Milestone 2).
	signaturePresent := r.Header.Get("Stripe-Signature") != ""
	log.Printf("webhooks/stripe: event_id=%q type=%q body_bytes=%d stripe_signature_present=%v remote_addr=%s",
		ev.ID, ev.Type, len(body), signaturePresent, r.RemoteAddr)

	w.WriteHeader(http.StatusNoContent)
}
