package main

import (
	"bytes"
	"context"
	"encoding/json"
	"integration-engine/internal/engine"
	"io"
	"net/http"
	"time"
)

type DownstreamClient interface {
	DeliverJob(ctx context.Context, job engine.Job) error
}

type httpDownstream struct {
	baseURL string
	client  *http.Client
}

func NewHTTPDownstream(baseURL string, client *http.Client) DownstreamClient {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &httpDownstream{
		baseURL: baseURL,
		client:  client,
	}
}

func (h *httpDownstream) DeliverJob(ctx context.Context, job engine.Job) error {
	body, err := json.Marshal(job)
	if err != nil {
		return permanentJobError("marshal job", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseURL, bytes.NewReader(body))
	if err != nil {
		return permanentJobError("build request", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return retryableJobError("downstream request", err, 0)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	return classifyHTTPStatus(resp.StatusCode)
}
