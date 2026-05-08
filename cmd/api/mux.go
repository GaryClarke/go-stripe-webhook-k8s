package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"runtime/debug"

	"integration-engine/internal/engine"
)

// healthResponse is used for probe-style JSON bodies (livez, readyz).
type healthResponse struct {
	Status string `json:"status"`
}

// newMux registers all API routes. Tests and production use the same wiring.
// When Milestone 2 adds config and shared dependencies, consider an application
// struct and passing it into newMux or using (app *application) routes(); see PLAN.md.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", handleLivez)
	mux.HandleFunc("GET /readyz", handleReadyz)
	mux.HandleFunc("POST /webhooks/stripe", handleStripeWebhook)
	return mux
}

func handleLivez(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	resp := healthResponse{Status: "ok"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("livez: encode response: %v", err)
	}
}

func handleReadyz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	resp := healthResponse{Status: "ok"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("readyz: encode response: %v", err)
	}
}

const maxStripeWebhookBody = 1 << 20 // 1 MiB; caps oversized abuse while fitting real Stripe events

func handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
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
