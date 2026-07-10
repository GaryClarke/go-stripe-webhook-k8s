package main

import (
	"encoding/json"
	"errors"
	"integration-engine/internal/engine"
	"io"
	"net/http"

	"github.com/stripe/stripe-go/v85"

	"integration-engine/internal/store"
)

// healthResponse is used for probe-style JSON bodies (livez, readyz).
type healthResponse struct {
	Status string `json:"status"`
}

func (app *App) handleLivez(w http.ResponseWriter, r *http.Request) {
	log := loggerFromContext(r.Context(), app.logger)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	resp := healthResponse{Status: "ok"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error(msgProbeEncodeError,
			"probe", "livez",
			"error", err.Error(),
		)
	}
}

func (app *App) handleReadyz(w http.ResponseWriter, r *http.Request) {
	log := loggerFromContext(r.Context(), app.logger)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	// DB readiness check
	if err := app.store.Ping(r.Context()); err != nil {
		log.Error(msgReadyzDBCheckFailed,
			"error", err.Error(),
		)
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	resp := healthResponse{Status: "ok"}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error(msgProbeEncodeError,
			"probe", "readyz",
			"error", err.Error(),
		)
	}
}

const maxStripeWebhookBody = 1 << 20 // 1 MiB; caps oversized abuse while fitting real Stripe events

func (app *App) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	log := loggerFromContext(r.Context(), app.logger)
	r.Body = http.MaxBytesReader(w, r.Body, maxStripeWebhookBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			log.Error(msgStripeBodyTooLarge,
				"max_bytes", maxStripeWebhookBody,
			)
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		}
		log.Error(msgStripeEventVerifyFailed,
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
		log.Error(msgStripeEventVerifyFailed,
			"reason", "verify_event",
			"error", err.Error(),
		)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var payload json.RawMessage
	if ev.Data != nil {
		payload = ev.Data.Raw
	}
	if len(payload) == 0 {
		payload = json.RawMessage("null")
	}

	job := engine.Job{
		StripeEventID: ev.ID,
		EventType:     string(ev.Type),
		Payload:       payload,
	}
	jobPayload, err := json.Marshal(job)
	if err != nil {
		log.Error(msgStripeEventProcessFailed,
			"event_id", ev.ID,
			"event_type", string(ev.Type),
			"error", err.Error(),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	claimed, err := app.store.AcceptEvent(r.Context(), ev.ID, string(ev.Type), jobPayload)
	if err != nil {
		log.Error(msgStripeEventProcessFailed,
			"event_id", ev.ID,
			"event_type", string(ev.Type),
			"error", err.Error(),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if claimed {
		log.Info(msgStripeEventAccepted,
			"event_id", ev.ID,
			"event_type", string(ev.Type),
		)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	es, err := app.store.Status(r.Context(), ev.ID)
	if err != nil {
		log.Error(msgStripeEventProcessFailed,
			"event_id", ev.ID,
			"event_type", string(ev.Type),
			"error", err.Error(),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	switch es.Status {
	case store.StatusAccepted, store.StatusProcessed:
		log.Info(msgStripeEventDuplicateSkipped,
			"event_id", ev.ID,
			"event_type", string(ev.Type),
		)
	case store.StatusProcessing:
		log.Info(msgStripeEventAlreadyProcessing,
			"event_id", ev.ID,
			"event_type", string(ev.Type),
		)
	default:
		log.Info(msgStripeEventDuplicateSkipped,
			"event_id", ev.ID,
			"event_type", string(ev.Type),
			"status", es.Status,
		)
	}
	w.WriteHeader(http.StatusNoContent)
}
