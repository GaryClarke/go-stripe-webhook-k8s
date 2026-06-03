package main

import (
	"context"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"strings"
)

type ctxKey int

const maxRequestIDLength int = 128

const (
	ctxKeyRequestID ctxKey = iota // iota = 0  →  ctxKey(0)
	ctxKeyLogger                  // iota = 1  →  ctxKey(1)
)

func resolveRequestID(r *http.Request) string {
	id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if id != "" && len(id) <= maxRequestIDLength {
		return id
	}
	return uuid.NewString()
}

func withRequestContext(ctx context.Context, requestID string, logger *slog.Logger) context.Context {
	ctx = context.WithValue(ctx, ctxKeyRequestID, requestID)
	ctx = context.WithValue(ctx, ctxKeyLogger, logger)
	return ctx
}

func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyRequestID).(string)
	return id
}

func loggerFromContext(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if logger, ok := ctx.Value(ctxKeyLogger).(*slog.Logger); ok && logger != nil {
		return logger
	}
	return fallback
}
