package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// parseLogLines splits test logger output into JSON objects (one per slog line).
func parseLogLines(t *testing.T, raw string) []map[string]any {
	t.Helper()

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	lines := strings.Split(raw, "\n")
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var fields map[string]any
		if err := json.Unmarshal([]byte(line), &fields); err != nil {
			t.Fatalf("log line is not valid JSON: %v\nline: %s", err, line)
		}
		out = append(out, fields)
	}
	return out
}

func findLogByMsg(lines []map[string]any, msg string) (map[string]any, bool) {
	for _, fields := range lines {
		if fields["msg"] == msg {
			return fields, true
		}
	}
	return nil, false
}

func TestRequestLog_logsStartedAndCompletedForNonProbe(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewJSONLogger(&buf, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhooks/stripe", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := RequestLog(logger, mux)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("response status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	lines := parseLogLines(t, buf.String())
	if len(lines) != 2 {
		t.Fatalf("got %d log lines, want 2 (started + completed); buf=%q", len(lines), buf.String())
	}

	started, ok := findLogByMsg(lines, msgRequestStarted)
	if !ok {
		t.Fatalf("missing %q in logs: %q", msgRequestStarted, buf.String())
	}
	if started["method"] != http.MethodPost {
		t.Fatalf("request_started method = %v, want POST", started["method"])
	}
	if started["path"] != "/webhooks/stripe" {
		t.Fatalf("request_started path = %v, want /webhooks/stripe", started["path"])
	}
	if _, ok := started["request_id"]; !ok {
		t.Fatal("request_started missing request_id")
	}

	completed, ok := findLogByMsg(lines, msgRequestCompleted)
	if !ok {
		t.Fatalf("missing %q in logs: %q", msgRequestCompleted, buf.String())
	}
	if completed["status"] != float64(http.StatusNoContent) {
		t.Fatalf("request_completed status = %v, want %d", completed["status"], http.StatusNoContent)
	}
	if _, ok := completed["duration_ms"]; !ok {
		t.Fatal("request_completed missing duration_ms")
	}
}

func TestRequestLog_skipsAccessLogsForProbe(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewJSONLogger(&buf, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestLog(logger, mux)
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("response status = %d, want %d", rec.Code, http.StatusOK)
	}

	lines := parseLogLines(t, buf.String())
	for _, fields := range lines {
		msg, _ := fields["msg"].(string)
		if msg == msgRequestStarted || msg == msgRequestCompleted {
			t.Fatalf("probe request must not emit access logs, got msg=%q", msg)
		}
	}
}

func TestRequestLog_honorsXRequestIDHeader(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := NewJSONLogger(&buf, nil)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequestLog(logger, mux)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-Id", "client-req-trace-1")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	lines := parseLogLines(t, buf.String())
	for _, fields := range lines {
		if fields["request_id"] != "client-req-trace-1" {
			t.Fatalf("request_id = %v, want client-req-trace-1 (msg=%v)", fields["request_id"], fields["msg"])
		}
	}
}
