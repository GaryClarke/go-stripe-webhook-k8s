package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsProbeRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{name: "livez GET", method: http.MethodGet, path: "/livez", want: true},
		{name: "readyz GET", method: http.MethodGet, path: "/readyz", want: true},
		{name: "livez GET with query", method: http.MethodGet, path: "/livez?probe=1", want: true},
		{name: "stripe webhook POST", method: http.MethodPost, path: "/webhooks/stripe", want: false},
		{name: "unknown GET", method: http.MethodGet, path: "/unknown", want: false},
		{name: "livez POST", method: http.MethodPost, path: "/livez", want: false},
		{name: "readyz POST", method: http.MethodPost, path: "/readyz", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if got := isProbeRequest(req); got != tt.want {
				t.Fatalf("isProbeRequest(%s %s) = %v, want %v", tt.method, req.URL.Path, got, tt.want)
			}
		})
	}
}

func TestResponseRecorder_WriteHeader_recordsStatus(t *testing.T) {
	t.Parallel()

	underlying := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: underlying, status: http.StatusOK}

	rec.WriteHeader(http.StatusNoContent)

	if rec.status != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.status, http.StatusNoContent)
	}
	if !rec.wroteHeader {
		t.Fatal("expected wroteHeader true after WriteHeader")
	}
	if underlying.Code != http.StatusNoContent {
		t.Fatalf("underlying status = %d, want %d", underlying.Code, http.StatusNoContent)
	}
}

func TestResponseRecorder_Write_implicitOKAndBytes(t *testing.T) {
	t.Parallel()

	underlying := httptest.NewRecorder()
	rec := &responseRecorder{ResponseWriter: underlying, status: http.StatusOK}

	const body = "ok"
	n, err := rec.Write([]byte(body))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(body) {
		t.Fatalf("Write n = %d, want %d", n, len(body))
	}
	if rec.status != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.status, http.StatusOK)
	}
	if !rec.wroteHeader {
		t.Fatal("expected wroteHeader true after Write")
	}
	if rec.bytes != len(body) {
		t.Fatalf("bytes = %d, want %d", rec.bytes, len(body))
	}
	if underlying.Body.String() != body {
		t.Fatalf("underlying body = %q, want %q", underlying.Body.String(), body)
	}
}
