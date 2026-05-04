# Go + AWS Integration Engine - Project Knowledge

This document captures the plan, architecture decisions, and key concepts for the Integration Engine project. It serves as the single source of truth for what we're building and how it works.

---

## 1. What We're Building

### Overview

A production-style, event-driven integration engine that:

- Receives webhooks from an external publisher (Stripe, initially)
- Validates and normalises payloads
- Enqueues events to a queue (SQS in production, in-memory locally)
- Processes them asynchronously
- Forwards them to a downstream third-party API
- Enforces idempotency and handles retries and rate limiting
- Uses DLQs and observability patterns

This is not a toy project. It mirrors real-world webhook → queue → worker → integration pipelines used in SaaS systems.

### Goals

1. **Cloud confidence** - Concurrency control, backpressure, failure isolation, retry strategies, event-driven architecture.
2. **Go in a real backend context** - HTTP, queue consumption, external APIs, testable services.
3. **Controlled concurrency** - SQS + Lambda reserved concurrency; no uncontrolled goroutine fan-out.
4. **Portfolio-grade example** - Documented architecture, interview talking point, proof of AWS fluency.

### Core Components

| Component | Description |
|-----------|-------------|
| **Webhook publisher** | External system sending events (Stripe CLI in production; fixtures/local harness for Phase 1). |
| **AWS infrastructure** | API Gateway, Ingest Lambda, SQS, Worker Lambda, DLQ (Phase 2). |
| **Go application** | Ingest handler (validate, normalise, enqueue) and Worker (consume, forward, retry). |
| **Downstream API** | Third-party receiver (Webhook.site, mock, or custom Lambda in Phase 3). |

### Event Types (v1)

- `invoice.payment_succeeded`
- `invoice.payment_failed`

---

## 2. Local vs Lambda: Two Entry Points, One Codebase

The same business logic runs in two environments. The difference is **how** the code is invoked, not **what** it does.

### Shared Logic

Both environments use the same core functions:

- **Ingest:** `HandleWebhook(ctx, body, headers) (status, responseBody, err)` - validate, normalise, enqueue.
- **Worker:** process one message: parse, idempotency check, forward to downstream, ack.

### Two Binaries, Two Entry Points

We do **not** have one binary that branches. We have **two main packages** that produce two executables:

| Binary | Built from | Used in | Entry point |
|--------|------------|---------|-------------|
| **ingest-local** | `cmd/ingest-local/` | Local dev | HTTP server listens on a port; each request → `HandleWebhook()`. |
| **bootstrap** (ingest) | `cmd/ingest/` | Lambda | `main()` calls `lambda.Start(handler)`; Lambda runtime invokes `handler(ctx, event)` per request. |
| **worker-local** | `cmd/worker-local/` | Local dev | Loop: receive from in-memory queue → worker logic. |
| **bootstrap** (worker) | `cmd/worker/` | Lambda | `main()` calls `lambda.Start(handler)`; Lambda runtime invokes `handler(ctx, sqsEvent)` per SQS batch. |

So:

- **Locally:** you run `ingest-local` and/or `worker-local` explicitly. No Lambda.
- **In production:** API Gateway invokes the ingest Lambda; SQS invokes the worker Lambda. No long-lived server in our code.

### Why a "Server" Exists Only Locally

- **Lambda:** There is no HTTP server in our code. API Gateway receives the HTTP request and **invokes** the Lambda with an **event payload**. Our code is just a function that receives that event and returns a response.
- **Local:** We need something to accept HTTP so we can send requests (e.g. `curl` or a test harness). So we run a normal Go HTTP server that calls `HandleWebhook()` for each request. That server exists **only for local development**.

### Configuration: How We Choose SQS vs In-Memory

- **Environment variables** (e.g. `QUEUE_BACKEND`, `SQS_QUEUE_URL`, `AWS_REGION`) determine behaviour.
- **Local:** `QUEUE_BACKEND=memory` (or unset). No SQS; use in-memory channel.
- **Production (Lambda):** Terraform sets `QUEUE_BACKEND=sqs`, `SQS_QUEUE_URL`, etc., on the Lambda.
- **Wiring:** In bootstrap/main we read config and construct either an SQS or in-memory implementation of the queue interface, then pass it into the handler. Same handler, different queue implementation.

---

## 3. How Lambda Receives a Request (No Magic)

Understanding how an HTTP request reaches our Go handler in Lambda removes the "magic."

### The Request Path

1. **Client** sends HTTP (e.g. `POST /webhook` with JSON body) to the API Gateway URL.
2. **API Gateway** receives the request. It does **not** forward raw HTTP to Lambda. It turns the request into a **Lambda event** (a JSON payload) and invokes the Lambda with that event.
3. **Lambda service** starts (or reuses) a container, runs our binary (`./bootstrap`), and sends the event payload to our process via the **Lambda Runtime API**.
4. **Our process** - `main()` called `lambda.Start(handler)`. The AWS Go SDK’s `lambda.Start` runs an event loop: it receives the event from the runtime, deserialises it into the type we expect (e.g. `events.APIGatewayV2HTTPRequest`), calls `handler(ctx, event)`, then sends our return value back to the runtime.
5. **API Gateway** receives the response from Lambda and turns it back into an HTTP response to the client.

So: **our code never sees TCP or HTTP.** It sees a single **event struct** per invocation and returns a **response struct**.

### The Two Explicit Connections

| Connection | Where it’s configured | What it does |
|------------|------------------------|--------------|
| **API Gateway → Lambda** | Terraform (or AWS console) | Route `POST /webhook` is tied to an **integration** whose target is our Lambda’s ARN. So "when this route is hit, invoke this Lambda." |
| **Event → our handler** | Our Go code | We pass the handler function to `lambda.Start(handler)`. The SDK calls that function for every event. There is no reflection or naming convention; we explicitly pass the function. |

### What the Handler Receives (HTTP API v2)

For API Gateway HTTP API, the event type is `events.APIGatewayV2HTTPRequest`. Our handler receives:

- `event.Body` - raw request body string (the webhook JSON).
- `event.Headers` - map of header names to values (e.g. `Stripe-Signature`).
- `event.RequestContext` - method, path, request ID, etc.

We parse `event.Body`, use headers for signature verification, run our logic, and return an `events.APIGatewayV2HTTPResponse` (status code, body, headers).

### Conventions

- **Lambda config:** The "handler" setting (e.g. `bootstrap`) is the name of the executable we deploy. Lambda runs that binary; the Go runtime then uses whatever we passed to `lambda.Start()`.
- **Event format:** Defined by API Gateway when it invokes Lambda. We choose the right event type in our handler signature (e.g. `APIGatewayV2HTTPRequest` for HTTP API).

---

## 4. Phase 1: Local-Only, No AWS or Stripe CLI

Phase 1 can be completed without:

- Any AWS account or SDK in the critical path (queue is in-memory).
- Stripe CLI or Stripe account.

We use:

- **Publisher:** Fixture files (e.g. `testdata/stripe-invoice-payment-succeeded.json`) and a small local HTTP client or `curl` to POST to our local ingest server.
- **Queue:** In-memory channel (`QUEUE_BACKEND=memory`).
- **Downstream:** Local mock (e.g. HTTP handler that logs the payload or returns configurable status codes).

Stripe event shapes are fixed in code and fixtures so we can build and test the full pipeline locally. When we later add Stripe CLI or deploy to AWS, we only change the entry point and configuration, not the core logic.

---

## 5. Build order (branches)

This section documents the **original Lambda-centric** branch sequence. Per-branch markdown now lives under **`docs/original-branches/NN-*.md`**. For the **Kubernetes / `cmd/api`** workstream, the living index is **[docs/branches/README.md](branches/README.md)** (still numbered branches + short write-ups; milestones in **[PLAN.md](../PLAN.md)**).

Work was done incrementally in **numbered branches** (git + branch doc). **§5 is the canonical sequence** for the historical Lambda phases—older ad-hoc “step 1–9” lists elsewhere should match this table.

### Phase 0: Minimal deploy

Full step-by-step (prerequisites, AWS, Terraform, CI): **[docs/phase-0-minimal-deploy.md](phase-0-minimal-deploy.md)** (steps 1–13).

| Branch | Doc | What it covers |
|--------|-----|----------------|
| **01** | [docs/original-branches/01-foundation.md](original-branches/01-foundation.md) | Go module, project layout, tooling |
| **02** | [docs/original-branches/02-api-gateway.md](original-branches/02-api-gateway.md) | API Gateway HTTP API, `GET /healthz` → Lambda |
| **03** | [docs/original-branches/03-s3-backend-ci.md](original-branches/03-s3-backend-ci.md) | Terraform remote state (S3 + DynamoDB lock) |

There is **no `04` branch doc**; numbering continues at **05** for Phase 1 app work (see [03 next branch](original-branches/03-s3-backend-ci.md)).

### Phase 1: Application (local queue, no SQS in the critical path yet)

| Branch | Doc | What it covers |
|--------|-----|----------------|
| **05** | [docs/original-branches/05-domain-types.md](original-branches/05-domain-types.md) | `StripeEvent`, `Job`, `testdata` fixture |
| **06** | [docs/original-branches/06-config.md](original-branches/06-config.md) | Env config, godotenv, `QUEUE_BACKEND` validation |
| **07** | [docs/original-branches/07-queue-abstraction.md](original-branches/07-queue-abstraction.md) | `Enqueuer` / `Consumer`, in-memory `MemoryQueue` |
| **08** | [docs/original-branches/08-queue-from-config.md](original-branches/08-queue-from-config.md) | `NewFromConfig`, `memory` vs `sqs` stub |
| **09** | [docs/original-branches/09-parse-stripe-event.md](original-branches/09-parse-stripe-event.md) | `ParseStripeEvent`, tests against `testdata` fixture |

### Proposed branches (Phase 1 remainder)

Rough scope for upcoming work—**adjust as you go**; add `docs/original-branches/NN-*.md` when each branch lands. Git branch names can mirror (e.g. `10-job-from-event`).

| Branch | Proposed scope |
|--------|----------------|
| **10** | **`StripeEvent` → `Job`** — mapping function (and tests); no queue yet, or optional tiny slice. |
| **11** | **Ingest core** — orchestrate parse → map → `Enqueue` with injected `Enqueuer`; table tests + fake `Enqueuer`; still no `cmd/*` if you want the smallest merge. |
| **12** | **HTTP entry (local +/or Lambda)** — `cmd/ingest-local` `POST /webhook` reading body into core; thin Lambda handler calling same core with `event.Body`. |
| **13** | **Downstream client** — interface + HTTP implementation (e.g. forward `Job` payload). |
| **14** | **Worker handler** — `Consumer.Consume` loop logic: parse `Job`, idempotency hook, call downstream, retry behaviour (as much as fits one branch). |
| **15** | **`cmd/worker-local`** — run consumer against in-memory queue; optional local end-to-end with ingest-local. |
| **16** | **Resilience and polish** — logging, retry tuning, error handling consistency across ingest/worker. |

**After Phase 2 (SQS, worker Lambda, DLQ):** dedicated **observability** pass (CloudWatch alarms, SNS, dashboard, billing alert)—not interleaved as tiny branches with early Phase 1 work.

### Phase 2 (later)

SQS, worker Lambda, DLQ, optional DynamoDB idempotency; Terraform and env for real queue backend.

### Branch index (quick links)

See **[docs/original-branches/README.md](original-branches/README.md)** for the historical Lambda branch list. For the K8s track, see **[docs/branches/README.md](branches/README.md)**.

---

## 6. Authentication and abuse prevention

How to protect the API and limit cost (app auth, IAM, throttling, billing). See [docs/auth-and-abuse-prevention.md](auth-and-abuse-prevention.md) for:

- Who can invoke the Lambda (only API Gateway / SQS; not the public).
- How to stop millions of invocations (API key, authorizer, throttling, billing alert).
- What we do per phase (healthz optional key; webhook Stripe signature + optional key).

---

## 7. References

- Stripe webhook event shape: [Stripe API - Events](https://stripe.com/docs/api/events).
- Lambda Go: [AWS Lambda Go](https://github.com/aws/aws-lambda-go); `lambda.Start(handler)` and `events.APIGatewayV2HTTPRequest` / `APIGatewayV2HTTPResponse`.
- API Gateway HTTP API payload: [Lambda proxy integration for HTTP APIs](https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-develop-integrations-lambda.html).

---

*This document is the project knowledge base. Update it as we lock in decisions and add phases.*
