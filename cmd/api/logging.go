package main

import (
	"github.com/google/uuid"
	"net/http"
	"strings"
)

const maxRequestIDLength int = 128

func resolveRequestID(r *http.Request) string {
	id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if id != "" && len(id) <= maxRequestIDLength {
		return id
	}
	return uuid.NewString()
}
