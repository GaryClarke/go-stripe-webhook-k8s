# Branch 02: Graceful shutdown

**Goal:** Run the API with `http.Server`, drain in-flight requests on **SIGINT** / **SIGTERM**, and exit cleanly so behaviour matches Kubernetes pod termination.

## What we added

- **`http.Server`** with **`Shutdown(ctx)`** instead of **`ListenAndServe`** alone on **`main`**.
- **Goroutine** runs **`ListenAndServe`** so **`main`** can block on signals ( **`ListenAndServe`** otherwise never returns until the server stops).
- **`signal.Notify`** on **`SIGINT`** (local Ctrl+C) and **`SIGTERM`** (typical cluster stop).
- **`context.WithTimeout`** (10s) for shutdown deadline; **`errors.Is(err, http.ErrServerClosed)`** after **`Shutdown`** so a successful drain is not treated as fatal.
- **`quit`** channel with **buffer 1** so at least one signal can be queued (see **`os/signal`** docs; avoids unbuffered rendezvous issues).

## Files changed

- **`cmd/api/main.go`** - server lifecycle, comments for learning.

## How to verify

```bash
go run ./cmd/api
# another terminal:
curl -sS http://localhost:8080/livez
# in first terminal:
Ctrl+C   # or: kill -TERM <pid>
```

Expect **"shutting down..."** in logs and process exit without **`listen:`** fatal from **`ErrServerClosed`**.

## Follow-ups

- **03-recover-panic** - see [03-recover-panic.md](03-recover-panic.md) (done on branch **`3-recover-panic`**).
- Align shutdown timeout with **`terminationGracePeriodSeconds`** when **`k8s/`** manifests exist.
- **`PORT`** from env (Milestone 2).
